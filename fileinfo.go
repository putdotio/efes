package main

import (
	"encoding/json"
	"os"
)

const fileInfoExt = ".info"

type FileInfo struct {
	Offset int64  `json:"offset"`
	Digest Digest `json:"digest"`
}

type Digest struct {
	Sha1  *sha1digest  `json:"sha1"`
	CRC32 *crc32digest `json:"crc32"`
}

func newFileInfo() *FileInfo {
	return &FileInfo{
		Digest: Digest{
			Sha1:  NewSha1(),
			CRC32: NewCRC32IEEE(),
		},
	}
}

func ReadFileInfo(path string) (fi *FileInfo, err error) {
	f, err := os.Open(path + fileInfoExt)
	if os.IsNotExist(err) {
		return newFileInfo(), nil
	}
	if err != nil {
		return
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&fi)
	return
}

func SaveFileInfo(path string, fi *FileInfo) error {
	f, err := os.Create(path + fileInfoExt)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(fi)
}

func DeleteFileInfo(path string) error {
	return os.Remove(path + fileInfoExt)
}
