package main

import (
	"os"

	"log"

	"net/http"

	"io"

	"fmt"

	"io/ioutil"
	"path/filepath"

	"./utils"
	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
)

func main() {
	address := ""
	if os.Getenv("DATABASE_ADDRESS") != "" {
		address = os.Getenv("DATABASE_ADDRESS")
	}
	err := utils.Open(os.Getenv("DATABASE_USERNAME"), os.Getenv("DATABASE_PASSWORD"), address, os.Getenv("DATABASE_NAME"))
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	router := gin.Default()
	router.GET("go-uploader/api/v1/files", getFileList)
	router.GET("go-uploader/api/v1/files/:filename", getFileInfo)
	router.POST("go-uploader/api/v1/files", postFile)
	router.DELETE("go-uploader/api/v1/files/:filename", deleteFile)
	router.PUT("go-uploader/api/v1/files/:filename", renameFile)
	router.GET("static/:filename", broadcastFile)

	router.Run(":" + os.Getenv("UPLOADER_PORT"))
}

func getFileList(c *gin.Context) {
	ID := c.GetHeader("id")
	if ID != "" {
		files := utils.GetFileList(ID)
		c.JSON(http.StatusOK, gin.H{"files": files})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{})
	}
}

func getFileInfo(c *gin.Context) {
	ID := c.GetHeader("id")
	if ID != "" {
		file, err := utils.GetFileByName(ID, c.Param("filename"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{})
		} else {
			c.JSON(http.StatusOK, file)
		}
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{})
	}
}

func postFile(c *gin.Context) {
	ID := c.GetHeader("id")
	if ID != "" {
		auth := c.PostForm("auth")
		if auth != "public" && auth != "internal" && auth != "private" {
			c.JSON(http.StatusBadRequest, gin.H{})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{})
			return
		}
		filename := header.Filename

		path := xid.New().String()

		out, err := os.Create(filepath.Join(os.Getenv("UPLOAD_FILE_PATH"), path))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{})
			return
		}
		defer out.Close()

		_, err = io.Copy(out, file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{})
		}

		var f utils.File
		f.UserID = ID
		f.Name = filename
		f.Path = path
		f.Auth = auth
		err = utils.InsertFileInfo(f)
		if err != nil {
			fmt.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{})
			return
		}

		c.JSON(http.StatusOK, f)

	} else {
		c.JSON(http.StatusUnauthorized, gin.H{})
	}
}

func deleteFile(c *gin.Context) {
	ID := c.GetHeader("id")
	if ID != "" {
		err := utils.DeleteFile(ID, c.Param("filename"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{})
		} else {
			c.JSON(http.StatusOK, gin.H{})
		}
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{})
	}
}

type RenamePost struct {
	NewName string `json:"newName" form:"newName" binding:"required"`
}

func renameFile(c *gin.Context) {
	ID := c.GetHeader("id")

	if ID != "" {
		var json RenamePost
		if c.BindJSON(&json) != nil {
			c.JSON(http.StatusBadRequest, gin.H{})
			return
		}

		err := utils.RenameFile(ID, c.Param("filename"), json.NewName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{})
		} else {
			c.JSON(http.StatusOK, gin.H{})
		}
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{})
	}
}

func broadcastFile(c *gin.Context) {
	ID := c.GetHeader("id")
	filename := c.Param("filename")
	var path string
	var err error

	if ID != "" {
		path, err = utils.GetFilePath(ID, filename)
	} else {
		path, err = utils.GetPublicFilePath(filename)
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}

	bytes, err := ioutil.ReadFile(filepath.Join(os.Getenv("UPLOAD_FILE_PATH"), path))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	mimeType := http.DetectContentType(bytes)

	c.Data(http.StatusOK, mimeType, bytes)
}
