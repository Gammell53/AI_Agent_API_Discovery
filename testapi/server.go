package testapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// User represents a user in our test API
type User struct {
	ID       int     `json:"id,omitempty"`
	Email    string  `json:"email"`
	Password string  `json:"password"`
	Name     string  `json:"name,omitempty"`
	Age      int     `json:"age,omitempty"`
	IsActive bool    `json:"isActive,omitempty"`
	Balance  float64 `json:"balance,omitempty"`
	Profile  Profile `json:"profile,omitempty"`
}

// Profile represents nested user profile data
type Profile struct {
	FirstName string   `json:"firstName"`
	LastName  string   `json:"lastName"`
	Hobbies   []string `json:"hobbies,omitempty"`
}

// StartTestServer starts a test API server on the specified port
func StartTestServer(port int) *gin.Engine {
	r := gin.Default()

	// Simple user creation endpoint - requires only email and password
	r.POST("/api/users", createUser)

	// Complex user creation - requires more fields and has nested objects
	r.POST("/api/users/complex", createComplexUser)

	// Product creation - different field types
	r.POST("/api/products", createProduct)

	// Array handling endpoint
	r.POST("/api/batch/users", createBatchUsers)

	return r
}

func createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if user.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}
	if user.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}

	// Simulate user creation
	user.ID = 1
	user.IsActive = true

	c.JSON(http.StatusCreated, user)
}

func createComplexUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if user.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}
	if user.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}
	if user.Profile.FirstName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "profile.firstName is required"})
		return
	}
	if user.Profile.LastName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "profile.lastName is required"})
		return
	}

	// Simulate user creation
	user.ID = 1
	user.IsActive = true

	c.JSON(http.StatusCreated, user)
}

// Product represents a product in our test API
type Product struct {
	ID         int      `json:"id,omitempty"`
	Name       string   `json:"name"`
	Price      float64  `json:"price"`
	SKU        string   `json:"sku"`
	InStock    bool     `json:"inStock"`
	Categories []string `json:"categories"`
}

func createProduct(c *gin.Context) {
	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if product.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if product.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "price must be greater than 0"})
		return
	}
	if product.SKU == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sku is required"})
		return
	}

	// Simulate product creation
	product.ID = 1

	c.JSON(http.StatusCreated, product)
}

func createBatchUsers(c *gin.Context) {
	var users []User
	if err := c.ShouldBindJSON(&users); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(users) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one user is required"})
		return
	}

	// Validate each user
	for i, user := range users {
		if user.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("user[%d].email is required", i)})
			return
		}
		if user.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("user[%d].password is required", i)})
			return
		}
	}

	// Simulate batch creation
	for i := range users {
		users[i].ID = i + 1
		users[i].IsActive = true
	}

	c.JSON(http.StatusCreated, users)
}
