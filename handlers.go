package handlers

import (

	//"go-auth/database"
	"net/http"

	//"github.com/golang-jwt/jwt/v4"

	//"regexp"

	"github.com/gin-gonic/gin"
	//"go.mongodb.org/mongo-driver/mongo"
)

// Обработчик главной страницы!!!
func IndexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Welcome",
	})
}
