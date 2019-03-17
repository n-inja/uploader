package utils

import (
	"database/sql"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"errors"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

type File struct {
	Name        string `json:"name" form:"name" binding:"required"`
	Path        string `json:"path" form:"name" binding:"required"`
	UserID      string `json:"userId" form:"userId" binding:"required"`
	AccessLevel string `json:"accessLevel" form:"accessLevel" binding:"required"`
	Mime        string `json:"mime" form:"mime"`
}

func init() {
	userName := os.Getenv("DATABASE_USERNAME")
	password := os.Getenv("DATABASE_PASSWORD")
	address := os.Getenv("DATABASE_ADDRESS")
	databaseName := os.Getenv("DATABASE_NAME")
	var err error
	db, err = sql.Open("mysql", userName+":"+password+"@tcp("+address+")/"+databaseName)

	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxIdleConns(0)
	err = initDB()
	if err != nil {
		log.Fatal(err)
	}
}

func Close() {
	db.Close()
}

func initDB() error {
	rows, err := db.Query("show tables like 'files'")
	if err != nil {
		return nil
	}
	defer rows.Close()
	if !rows.Next() {
		_, err = db.Exec("create table files (name varchar(256) NOT NULL PRIMARY KEY, path varchar(20) NOT NULL, user_id varchar(32) NOT NULL, access_level varchar(10) NOT NULL, mime varchar(32) NOT NULL, index(user_id))")
		if err != nil {
			return err
		}
	}

	return nil
}

func GetFileList(ID string) ([]File, error) {
	rows, err := db.Query("select count(*) from files where access_level = 'public' OR access_level = 'internal' OR access_level = 'private' AND user_id = ?", ID)
	if err != nil {
		return nil, err
	}
	var count int32
	rows.Next()
	rows.Scan(&count)
	rows.Close()

	files := make([]File, 0)

	rows, err = db.Query("select name, path, user_id, access_level from files where access_level = 'public' OR access_level = 'internal' OR access_level = 'private' AND user_id = ?", ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for i := 0; rows.Next(); i++ {
		var f File
		err := rows.Scan(&f.Name, &f.Path, &f.UserID, &f.AccessLevel)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}

	return files, nil
}

func GetFileByName(ID, name string) (File, error) {
	rows, err := db.Query("select name, path, user_id, access_level from files where name = ? and (access_level != 'private' or access_level = 'private' and user_id = ?)", name, ID)
	var ret File
	if err != nil {
		return ret, err
	}
	if !rows.Next() {
		rows.Close()
		return ret, errors.New("not found")
	}
	rows.Scan(&ret.Name, &ret.Path, &ret.UserID, &ret.AccessLevel)
	defer rows.Close()
	return ret, nil
}

func InsertFileInfo(file File) error {
	_, err := db.Exec("insert into files values(?, ?, ?, ?, ?)", file.Name, file.Path, file.UserID, file.AccessLevel, file.Mime)
	if err != nil {
		return err
	}
	return nil
}

func DeleteFile(ID, filename string) error {
	if ID == "root" {
		_, err := db.Exec("delete from files where name = ?", filename)
		if err != nil {
			return err
		}
	} else {
		_, err := db.Exec("delete from files where name = ? and user_id = ?", filename, ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func RenameFile(ID, filename, newFilename, newAccessLevel string) error {
	if newAccessLevel == "" && newFilename == "" {
		return errors.New("filename must be ")
	} else if newAccessLevel != "" && newFilename == "" {
		if ID == "root" {
			_, err := db.Exec("update files set access_level = ? where name = ? and (access_level != 'private' or user_id = 'root')", newAccessLevel, filename)
			if err != nil {
				return err
			}
		} else {
			_, err := db.Exec("update files set access_level = ? where name = ? and user_id = ?", newAccessLevel, filename, ID)
			if err != nil {
				return err
			}
		}
	} else if newAccessLevel == "" && newFilename != "" {
		if ID == "root" {
			_, err := db.Exec("update files set name = ? where name = ? and (access_level != 'private' or user_id = 'root')", newFilename, filename)
			if err != nil {
				return err
			}
		} else {
			_, err := db.Exec("update files set name = ? where name = ? and user_id = ?", newFilename, filename, ID)
			if err != nil {
				return err
			}
		}
	} else {
		if ID == "root" {
			_, err := db.Exec("update files set name = ?, access_level = ? where name = ? and (access_level != 'private' or user_id = 'root')", newFilename, newAccessLevel, filename)
			if err != nil {
				return err
			}
		} else {
			_, err := db.Exec("update files set name = ?, access_level = ? where name = ? and user_id = ?", newFilename, newAccessLevel, filename, ID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func GetFilePath(ID, filename string) (path, mime string, err error) {
	rows, err := db.Query("select path, mime from files where name = ? and (access_level = 'public' or access_level = 'internal' or access_level = 'private' and user_id = ?)", filename, ID)
	if err != nil {
		return
	}
	if !rows.Next() {
		err = errors.New("file not found")
		return
	}
	rows.Scan(&path, &mime)
	return
}

func GetPublicFilePath(filename string) (path, mime string, err error) {
	rows, err := db.Query("select path, mime from files where name = ? and access_level = 'public'", filename)
	if err != nil {
		return
	}
	if !rows.Next() {
		err = errors.New("file not found")
		return
	}
	rows.Scan(&path, &mime)
	return
}

func UpdateMime() {
	rows, err := db.Query("select path from files")
	if err != nil {
		log.Fatal(err)
	}
	base := os.Getenv("UPLOAD_FILE_PATH")
	for rows.Next() {
		var path string
		rows.Scan(&path)
		p := filepath.Join(base, path)
		bytes, err := ioutil.ReadFile(p)
		if err != nil {
			log.Fatal(err)
		}
		mime := http.DetectContentType(bytes)
		_, err = db.Exec("update files set mime = ? where path = ?", mime, path)
		if err != nil {
			log.Fatal(err)
		}
		m, err := exec.Command("bash", "-c", "gzip -c "+p+" > "+p+".gz").CombinedOutput()
		if err != nil {
			log.Fatal(m, err)
		}
	}
}
