package utils

import (
	"database/sql"

	"errors"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

type File struct {
	Name   string `json:"name" form:"name" binding:"required"`
	Path   string `json:"path" form:"name" binding:"required"`
	UserID string `json:"userId" form:"userId" binding:"required"`
	Auth   string `json:"auth" form:"auth" binding:"required"`
}

func Open(userName, password, address, databaseName string) error {
	var err error
	db, err = sql.Open("mysql", userName+":"+password+"@"+address+"/"+databaseName)
	if err != nil {
		return err
	}
	return initDB()
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
		_, err = db.Exec("create table files (name varchar(256) NOT NULL PRIMARY KEY, path varchar(20) NOT NULL, user_id varchar(32) NOT NULL, auth varchar(10) NOT NULL, index(user_id))")
		if err != nil {
			return err
		}
	}

	return nil
}

func GetFileList(ID string) []File {
	rows, err := db.Query("select count(*) from files where auth = 'public' OR auth = 'internal' OR auth = 'private' AND user_id = ?", ID)
	if err != nil {
		return []File{}[:]
	}
	var count int32
	rows.Next()
	rows.Scan(&count)
	rows.Close()

	if count > 1000 {
		count = 1000
	}

	var ret [1000]File

	rows, err = db.Query("select name, path, user_id, auth from files where auth = 'public' OR auth = 'internal' OR auth = 'private' AND user_id = ?", ID)
	if err != nil {
		return []File{}[:]
	}
	defer rows.Close()

	for i := 0; rows.Next(); i++ {
		err := rows.Scan(&ret[i].Name, &ret[i].Path, &ret[i].UserID, &ret[i].Auth)
		if err != nil {
			return []File{}[:]
		}
	}

	return ret[0:count]
}

func GetFileByName(ID, name string) (File, error) {
	rows, err := db.Query("select name, path, user_id, auth from files where name = ? and (auth != 'private' or auth = 'private' and user_id = ?)", name, ID)
	var ret File
	if err != nil {
		return ret, err
	}
	if !rows.Next() {
		rows.Close()
		return ret, errors.New("not found")
	}
	rows.Scan(&ret.Name, &ret.Path, &ret.UserID, &ret.Auth)
	defer rows.Close()
	return ret, nil
}

func InsertFileInfo(file File) error {
	_, err := db.Exec("insert into files values(?, ?, ?, ?)", file.Name, file.Path, file.UserID, file.Auth)
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

func RenameFile(ID, filename, newFilename string) error {
	if ID == "root" {
		_, err := db.Exec("update files set name = ? where name = ? and (auth != 'private' or user_id = 'root')", newFilename, filename)
		if err != nil {
			return err
		}
	} else {
		_, err := db.Exec("update files set name = ? where name = ? and user_id = ?", newFilename, filename, ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetFilePath(ID, filename string) (string, error) {
	rows, err := db.Query("select path from files where name = ? and (auth = 'public' or auth = 'internal' or auth = 'private' and user_id = ?)", filename, ID)
	if err != nil {
		return "", err
	}
	if !rows.Next() {
		return "", errors.New("file not found")
	}
	var path string
	rows.Scan(&path)
	return path, nil
}

func GetPublicFilePath(filename string) (string, error) {
	rows, err := db.Query("select path from files where name = ? and auth = 'public'", filename)
	if err != nil {
		return "", err
	}
	if !rows.Next() {
		return "", errors.New("file not found")
	}
	var path string
	rows.Scan(&path)
	return path, nil
}
