package cmd

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/h2non/filetype"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/spf13/cobra"
)

type FileInfo struct {
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	Mode         string `json:"mode"`
	UserId       uint32 `json:"user_id"`
	GroupId      uint32 `json:"group_id"`
	ChangeTime   int64  `json:"changed_time"`
	ModifiedTime int64  `json:"modified_time"`
	AccessTime   int64  `json:"access_time"`
	Error        string `json:"error"`
	Context      string `json:"context"`
	MD5          string `json:"md5"`
	SHA1         string `json:"sha1"`
	SHA256       string `json:"sha256"`
	SHA512       string `json:"sha512"`
	MimeType     string `json:"mime_type"`
}

type Job struct {
	FilePath string
	FileInfo os.FileInfo
	Hash     bool
}

var hashOption bool

func getMimeType(buf []byte) (string, error) {
	kind, err := filetype.Match(buf)
	if err != nil {
		return "", err
	}

	if kind.MIME.Value != "" {
		return kind.MIME.Value, nil
	}

	return http.DetectContentType(buf), nil
}

func init() {
	rootCmd.AddCommand(findCmd)

	findCmd.PersistentFlags().BoolVarP(&hashOption, "hash", "H", false,
		"Check the file hash")
}

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "List files in a given folder",
	Long:  `List files in a given folder`,
	Run:   find,
}

func processFile(filePath string, fileInfo os.FileInfo, getHash bool) FileInfo {
	f := FileInfo{
		Path:         filePath,
		Size:         fileInfo.Size(),
		Mode:         fileInfo.Mode().String(),
		ModifiedTime: fileInfo.ModTime().Unix(),
	}

	stat := fileInfo.Sys().(*syscall.Stat_t)
	f.AccessTime = time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec)).Unix()
	f.ChangeTime = time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec)).Unix()
	f.UserId = stat.Uid
	f.GroupId = stat.Gid
	label, err := selinux.FileLabel(filePath)
	if err == nil {
		f.Context = label
	}

	if getHash {
		// no hash for /proc/
		if strings.HasPrefix(filePath, "/proc/") || strings.HasPrefix(filePath, "/sys/") || strings.HasPrefix(filePath, "/system/") {
			return f
		}

		file, err := os.Open(filePath)
		if err != nil {
			return f
		}
		defer file.Close()

		buf := make([]byte, f.Size)
		_, err = file.Read(buf)
		if err != nil {
			return f
		}

		mimeType, err := getMimeType(buf)
		if err == nil {
			f.MimeType = mimeType
		}

		hashes := []hash.Hash{
			md5.New(),
			sha1.New(),
			sha256.New(),
			sha512.New(),
		}

		for _, h := range hashes {
			h.Write(buf)
		}

		f.MD5 = hex.EncodeToString(hashes[0].Sum(nil))
		f.SHA1 = hex.EncodeToString(hashes[1].Sum(nil))
		f.SHA256 = hex.EncodeToString(hashes[2].Sum(nil))
		f.SHA512 = hex.EncodeToString(hashes[3].Sum(nil))

	}

	return f
}

func worker(jobChan chan Job, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobChan {
		f := processFile(job.FilePath, job.FileInfo, job.Hash)
		jsonData, err := json.Marshal(&f)
		if err != nil {
			continue
		}
		fmt.Println(string(jsonData))
	}
}

// Execute the command
func find(cmd *cobra.Command, args []string) {
	var target_path string
	if len(args) == 0 {
		target_path = "/"
	} else {
		target_path = args[0]
	}

	if _, err := os.Stat(target_path); os.IsNotExist(err) {
		return
	}

	jobChan := make(chan Job)
	wg := new(sync.WaitGroup)

	np_proc := math.Max(1.0, float64(runtime.NumCPU()-3))

	for i := 0; i < int(np_proc); i++ {
		wg.Add(1)
		go worker(jobChan, wg)
	}

	err := filepath.Walk(target_path,
		func(path string, info os.FileInfo, err error) error {
			if err == nil {
				if !info.IsDir() {
					jobChan <- Job{
						FilePath: path,
						FileInfo: info,
						Hash:     hashOption,
					}
				}
			}
			return nil
		})
	if err != nil {
		log.Fatal(err)
	}
	close(jobChan)
	wg.Wait()
}
