package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func _string(context *gin.Context) {
	context.String(http.StatusOK, "hello world!")
}
func _json(c *gin.Context) {
	// 1. json in response to struct
	type UserInfo struct {
		UserName string `json:"user_name"`
		Age int32 `json:"age"`
		Password int32 `json:"-"` // ignore
	}
	user := UserInfo{"eland", 21, 12345}
	c.JSON(200, user)
	// 2. json in response to map
	userMap := map[string]string{
		"user_name":"eland",
		"age":"22",
	}
	c.JSON(200, userMap)
	// 3. in response to JSON directly
	c.JSON(200, gin.H{"username":"eland", "age":23})
}
func _html(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{})
}

func main() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/index.html")
	router.Static("/style", "/static/*")
	router.GET("/index", _string)
	router.GET("/json", _json)
	router.GET("/html", _html)
	//router.GET("/xml", _xml)
	//router.GET("/yaml", _yaml)
	http.ListenAndServe(":8080", router)
}


