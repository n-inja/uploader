package main

import (
	"log"
	"os"
	"os/exec"
	"strings"

	"net/http"

	"io"

	"io/ioutil"
	"path/filepath"

	"./utils"
	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
)

var uploadFilePath string

func init() {
	uploadFilePath = os.Getenv("UPLOAD_FILE_PATH")
}

func main() {
	if os.Getenv("UPDATE_MIME") != "" {
		utils.UpdateMime()
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("go-uploader/api/v1/files", getFileList)
	router.GET("go-uploader/api/v1/files/:filename", getFileInfo)
	router.POST("go-uploader/api/v1/files", postFile)
	router.DELETE("go-uploader/api/v1/files/:filename", deleteFile)
	router.PUT("go-uploader/api/v1/files/:filename", renameFile)
	router.GET("static/:filename", broadcastFile)

	log.Fatal(router.Run(":" + os.Getenv("UPLOADER_PORT")))
}

func getFileList(c *gin.Context) {
	ID := c.GetHeader("id")
	if ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{})
		return
	}

	files, err := utils.GetFileList(ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": files})
}

func getFileInfo(c *gin.Context) {
	ID := c.GetHeader("id")
	if ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{})
		return
	}

	file, err := utils.GetFileByName(ID, c.Param("filename"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}

	c.JSON(http.StatusOK, file)
}

func postFile(c *gin.Context) {
	ID := c.GetHeader("id")
	if ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{})
		return
	}

	accessLevel := c.PostForm("accessLevel")
	if accessLevel != "public" && accessLevel != "internal" && accessLevel != "private" {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	filename := header.Filename

	path := xid.New().String()

	if c.PostForm("name") != "" {
		filename = c.PostForm("name")
	}

	outpath := filepath.Join(uploadFilePath, path)
	out, err := os.Create(outpath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	_, err = io.Copy(out, file)
	out.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	message, err := exec.Command("bash", "-c", "gzip -c "+outpath+" > "+outpath+".gz").CombinedOutput()

	if err != nil {
		log.Println(err, message)
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	var f utils.File
	f.UserID = ID
	f.Name = filename
	f.Path = path
	f.AccessLevel = accessLevel
	f.Mime, err = getMime(outpath)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	err = utils.InsertFileInfo(f)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	c.JSON(http.StatusOK, f)
}

func getMime(path string) (string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return http.DetectContentType(bytes), nil
}

func deleteFile(c *gin.Context) {
	ID := c.GetHeader("id")
	if ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{})
		return
	}

	err := utils.DeleteFile(ID, c.Param("filename"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

type RenamePost struct {
	NewName        string `json:"newName" form:"newName"`
	NewAccessLevel string `json:"newAccessLevel" form:"newAccessLevel"`
}

func renameFile(c *gin.Context) {
	ID := c.GetHeader("id")
	if ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{})
		return
	}

	var json RenamePost
	if c.BindJSON(&json) != nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}
	if json.NewAccessLevel != "" && json.NewAccessLevel != "public" && json.NewAccessLevel != "internal" && json.NewAccessLevel != "private" {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	err := utils.RenameFile(ID, c.Param("filename"), json.NewName, json.NewAccessLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func broadcastFile(c *gin.Context) {
	ID := c.GetHeader("id")
	filename := c.Param("filename")
	var path, mime string
	var err error

	if ID != "" {
		path, mime, err = utils.GetFilePath(ID, filename)
	} else {
		path, mime, err = utils.GetPublicFilePath(filename)
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{})
		return
	}

	serveFile(c, filepath.Join(uploadFilePath, path), mime)
}

func serveFile(c *gin.Context, path, mime string) {
	gzipOK := strings.Contains(c.GetHeader("Accept-Encoding"), "gzip")
	var bytes []byte
	var err error
	if gzipOK {
		bytes, err = ioutil.ReadFile(path + ".gz")
		c.Header("Content-Encoding", "gzip")
	} else {
		bytes, err = ioutil.ReadFile(path)
	}
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	c.Data(http.StatusOK, mime, bytes)
}
