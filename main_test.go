package main

import (
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-playground/assert/v2"
	_ "github.com/go-sql-driver/mysql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 1. 单元测试

func TestUserRegistration(t *testing.T) {
	// 测试用户注册功能，验证数据库中是否正确添加了用户，并且确认返回了正确的HTTP状态代码和消息。
}
func TestWebSocketConnection(t *testing.T) {
	// 测试WebSocket连接是否成功建立，并且在完成后是否正确关闭。
}

func TestBroadcastMessage(t *testing.T) {
	// 测试用户消息是否正确广播到所有连接的客户端。
}

func TestChatWithGPT(t *testing.T) {
	// 测试与GPT对话功能，确认是否返回了有效地响应。
}

func TestLoginPage(t *testing.T) {
	router := gin.Default()
	router.GET("/login", func(c *gin.Context) {
		c.HTML(200, "login.html", nil)
	})

	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("Expected HTTP response code 200, but got: %v", w.Code)
	}
}

func TestChatPage(t *testing.T) {
	router := gin.Default()
	router.GET("/chat", func(c *gin.Context) {
		c.HTML(200, "chat.html", nil)
	})

	req := httptest.NewRequest("GET", "/chat", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("Expected HTTP response code 200, but got: %v", w.Code)
	}
}

func TestDatabaseConnection(t *testing.T) {
	db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/chatroom")
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	err = db.Ping()
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}
}

func TestGetUserPassword(t *testing.T) {
	// Connect to the test database
	db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/test_db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Insert a test user
	_, err = db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", "test_user", "test_password")
	if err != nil {
		t.Fatal(err)
	}

	// Test the getUserPassword function
	password, err := getUserPassword(db, "test_user")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "test_password", password)

	// Clean up the test user
	_, err = db.Exec("DELETE FROM users WHERE username = ?", "test_user")
	if err != nil {
		t.Fatal(err)
	}
}

func getUserPassword(db *sql.DB, username string) (string, error) {
	var password string
	err := db.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&password)
	if err != nil {
		if err == sql.ErrNoRows {
			// There's no user with the provided username in the database
			return "", errors.New("user not found")
		} else {
			// There was an error executing the query
			return "", err
		}
	}
	return password, nil
}

// 2. 集成测试

func TestIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Mock the database
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// Set your expectations
	mock.ExpectExec("INSERT INTO users").
		WithArgs("testuser", "testpass").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery("SELECT password FROM users").
		WithArgs("testuser").
		WillReturnRows(sqlmock.NewRows([]string{"password"}).AddRow("testpass"))

	// Start the server
	r := setupRouter(db) // Assume you have a setupRouter function that returns a *gin.Engine

	// Simulate a user registration
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/register", strings.NewReader("username=testuser&password=testpass&confirm_password=testpass"))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	// Check if the status code is correct
	assert.Equal(t, http.StatusMovedPermanently, w.Code)

	// Simulate a user login
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/login", strings.NewReader("username=testuser&password=testpass"))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	// Check if the status code is correct
	assert.Equal(t, http.StatusMovedPermanently, w.Code)

	// You should add more tests here to simulate sending messages and receiving AI responses...
}

func setupRouter(db *sql.DB) *gin.Engine {
	r := gin.Default()
	// Set up your routes
	r.POST("/login", func(c *gin.Context) {
		// Your login logic here
	})
	// More routes...
	return r
}

// 错误测试

func TestGetUserPasswordError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("An error '%s' was not expected when opening a stub database connection", err)
	}

	// Close the mock database connection
	defer db.Close()

	// Now we need to setup our expectations
	mock.ExpectQuery("SELECT password FROM users WHERE username = ?").
		WithArgs("non_existent_user").
		WillReturnError(sql.ErrNoRows) // Simulate a "no rows in result set" error

	password, err := getUserPassword(db, "non_existent_user")
	if err == nil {
		t.Errorf("Expected an error when fetching password for non-existent user")
	}
	if err.Error() != "user not found" {
		t.Errorf("Expected 'user not found' error, got '%s'", err.Error())
	}
	if password != "" {
		t.Errorf("Expected an empty password for non-existent user, got '%s'", password)
	}

	// Simulate a database error
	mock.ExpectQuery("SELECT password FROM users WHERE username = ?").
		WithArgs("database_error").
		WillReturnError(errors.New("database error"))

	password, err = getUserPassword(db, "database_error")
	if err == nil {
		t.Errorf("Expected an error when fetching password with a database error")
	}
	if err.Error() != "database error" {
		t.Errorf("Expected 'database error', got '%s'", err.Error())
	}
	if password != "" {
		t.Errorf("Expected an empty password for user with database error, got '%s'", password)
	}

	// We need to ensure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("There were unfulfilled expectations: %s", err)
	}
}
