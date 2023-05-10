package main

import (
	//"bytes"
	//"encoding/json"
	//"io/ioutil"
	//"log"

	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	msgChan = make(chan UserMessage) // 定义全局的消息通道

	// 建立一个WebSocket连接的映射，并使用互斥锁进行保护
	conns = struct {
		sync.RWMutex
		m map[*websocket.Conn]string
	}{m: make(map[*websocket.Conn]string)}
)

const (
	openaiURL    = "https://api.openai.com/v1/chat/completions"
	openaiAPIKey = ""
)

type UserMessage struct {
	Sender  string
	Content string
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestBody struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ResponseBody struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      Message
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func main() {
	// 打开一个数据库连接
	db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/chatroom")
	if err != nil {
		fmt.Println(err)
		return
	}

	// Close the database connection
	defer db.Close()

	// Test the database connection
	err = db.Ping()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Connected to MySQL database!")
	// 打开一个路由
	router := gin.Default()
	router.LoadHTMLGlob("templates/*.html")
	router.Static("/static", "./static")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	// sign in Interface
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

		// 查询用户信息
		var storedPassword string
		query := "SELECT password FROM users WHERE username = ?"
		err := db.QueryRow(query, username).Scan(&storedPassword)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query user information"})
			}
			return
		}

		// 验证用户密码
		if password != storedPassword {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}

		// 设置Cookie
		usernameCookie := http.Cookie{
			Name:  "username",
			Value: username,
			Path:  "/",
		}
		http.SetCookie(c.Writer, &usernameCookie)

		c.Redirect(http.StatusMovedPermanently, "/chat")
	})

	// sign up 	Interface
	router.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", nil)
	})

	router.POST("/register", func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")
		confirm_password := c.PostForm("confirm_password")

		// 判断用户名和密码是否为空
		if username == "" || password == "" || confirm_password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
			return
		}
		// 判断密码是否一致
		if password != confirm_password {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Passwords do not match"})
			return
		}

		// 将用户注册信息保存到数据库中
		insertQuery := "INSERT INTO users (username, password) VALUES (?, ?);"
		_, err := db.Exec(insertQuery, username, password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
			return
		}
		// 设置Cookie
		usernameCookie := http.Cookie{
			Name:  "username",
			Value: username,
			Path:  "/",
		}
		http.SetCookie(c.Writer, &usernameCookie)
		// 注册成功，跳转到聊天室页面
		c.Redirect(http.StatusMovedPermanently, "/chat")
	})

	// chat Interface
	router.GET("/chat", func(c *gin.Context) {
		c.HTML(http.StatusOK, "chat.html", nil)
	})

	//
	router.GET("/ws", func(c *gin.Context) {
		wshandler(c.Writer, c.Request)
	})

	// 在 main 函数中启动消息广播和消息保存到数据库的协程
	go func() {
		for {
			msg := <-msgChan

			// 将消息存储到数据库中
			insertQuery := "INSERT INTO messages (sender, message) VALUES (?, ?)"
			_, err := db.Exec(insertQuery, msg.Sender, msg.Content)
			if err != nil {
				fmt.Printf("Failed to insert message to database: %v", err)
				continue
			}

			// 广播消息给所有连接的 WebSocket 客户端
			conns.RLock()
			for conn := range conns.m {
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Content)); err != nil {
					fmt.Printf("Failed to broadcast message: %+v\n", err)
				}
			}
			conns.RUnlock()
		}
	}()

	router.Run(":8080")
}

func wshandler(w http.ResponseWriter, r *http.Request) {
	// 升级连接为 WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to set websocket upgrade: %+v\n", err)
		return
	}

	// 在这个位置获取客户端IP地址和用户名
	ip := r.RemoteAddr
	cookie, err := r.Cookie("username")
	if err != nil {
		fmt.Println("Error getting username from cookie:", err)
		return
	}
	username := cookie.Value

	// 将新的 WebSocket 连接添加到映射中
	conns.Lock()
	conns.m[conn] = username
	conns.Unlock()

	// 在连接建立后发送在线用户列表
	sendUserList()

	defer func() {
		// 当连接断开时，从映射中移除
		conns.Lock()
		delete(conns.m, conn)
		conns.Unlock()

		// 在连接断开后发送在线用户列表
		sendUserList()
	}()

	// 读取消息信息
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		message := UserMessage{
			Sender:  username,
			Content: fmt.Sprintf("%s (%s): %s", username, ip, string(msg)),
		}

		// 将消息发送到全局消息通道
		msgChan <- message

		//fmt.Println(message.Content)
		//fmt.Println(chatWithGPT(message.Content).Content)
		chatWithGPT(string(msg))

	}

}

func sendUserList() {
	conns.RLock()
	defer conns.RUnlock()

	var userList []string
	for _, username := range conns.m {
		userList = append(userList, username)
	}

	userListMessage := strings.Join(userList, ", ")
	for conn := range conns.m {
		if err := conn.WriteMessage(websocket.TextMessage, []byte("Online users: "+userListMessage)); err != nil {
			fmt.Printf("Failed to send user list: %+v\n", err)
		}
	}
}

//	func broadcast(username, ip, msg string) {
//		conns.RLock()
//		defer conns.RUnlock()
//
//		message := fmt.Sprintf("%s (%s): %s", username, ip, msg)
//		for conn := range conns.m {
//			if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
//				fmt.Printf("Failed to broadcast message: %+v\n", err)
//			}
//		}
//	}
func chatWithGPT(msg string) UserMessage {
	reqBody := RequestBody{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{Role: "user", Content: msg},
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest("POST", openaiURL, bytes.NewBuffer(payload))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openaiAPIKey)

	proxyURL, err := url.Parse("http://localhost:7890") // Replace with your proxy server URL
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	fmt.Println("Response Status:", resp.Status)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Response Body:", string(body))

	var respBody ResponseBody
	err = json.Unmarshal(body, &respBody)
	if err != nil {
		log.Fatal(err)
	}

	//fmt.Printf("ID: %s\n", respBody.ID)
	//fmt.Printf("Object: %s\n", respBody.Object)
	//fmt.Printf("Created: %d\n", respBody.Created)

	message := UserMessage{"AI", respBody.Choices[0].Message.Content}
	fmt.Println(msg)
	for i, choice := range respBody.Choices {
		fmt.Printf("Choice %d:\n", i)
		fmt.Printf("  Index: %d\n", choice.Index)
		fmt.Printf("  Finish Reason: %s\n", choice.FinishReason)
		fmt.Printf("  Message Role: %s\n", choice.Message.Role)
		fmt.Printf("  Message Content: %s\n", choice.Message.Content)

		message = UserMessage{"AI", respBody.Choices[0].Message.Content}
		messageofGPT := UserMessage{
			Sender:  "AI",
			Content: fmt.Sprintf("%s (%s): %s", "AI", "0.0.0.0", message),
		}
		msgChan <- messageofGPT
	}
	//fmt.Printf("Usage:\n")
	//fmt.Printf("  Prompt Tokens: %d\n", respBody.Usage.PromptTokens)
	//fmt.Printf("  Completion Tokens: %d\n", respBody.Usage.CompletionTokens)
	//fmt.Printf("  Total Tokens: %d\n", respBody.Usage.TotalTokens)

	//message := UserMessage{"AI", respBody.Choices[0].Message.Content}

	return message
}
