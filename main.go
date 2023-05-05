package main

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	msgChan = make(chan string) // 定义全局的消息通道

	// 建立一个WebSocket连接的映射，并使用互斥锁进行保护
	conns = struct {
		sync.RWMutex
		m map[*websocket.Conn]bool
	}{m: make(map[*websocket.Conn]bool)}
)

func main() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*.html")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", nil)
	})

	router.POST("/login", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		if username == "" || password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
			return
		}

		c.Redirect(http.StatusMovedPermanently, "/chat")
	})

	router.GET("/chat", func(c *gin.Context) {
		c.HTML(http.StatusOK, "chat.html", nil)
	})

	router.GET("/ws", func(c *gin.Context) {
		wshandler(c.Writer, c.Request)
	})

	// 启动消息广播协程
	go func() {
		for {
			msg := <-msgChan
			broadcast(msg)
		}
	}()

	router.Run(":8080")
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to set websocket upgrade: %+v\n", err)
		return
	}

	// 将新的WebSocket连接添加到映射中
	conns.Lock()
	conns.m[conn] = true
	conns.Unlock()

	defer func() {
		// 当连接断开时，从映射中移除
		conns.Lock()
		delete(conns.m, conn)
		conns.Unlock()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// 将消息发送到全局的消息通道
		msgChan <- string(msg)
	}
}

func broadcast(msg string) {
	conns.RLock()
	defer conns.RUnlock()

	for conn := range conns.m {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			fmt.Printf("Failed to broadcast message: %+v\n", err)
		}
	}
}
