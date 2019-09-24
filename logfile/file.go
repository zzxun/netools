package logfile

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// COPY from logrus_mate/hooks/file for custom modify.

// FileLogConifg for setting file path, rotate strategy
type FileLogConifg struct {
	Filename string       `json:"filename"`
	Maxlines int          `json:"maxlines"`
	Maxsize  int          `json:"maxsize"`
	Daily    bool         `json:"daily"`
	Maxdays  int64        `json:"maxdays"`
	Rotate   bool         `json:"rotate"`
	Level    logrus.Level `json:"level"`
}

// FileLogWriter implements LoggerInterface.
// It writes messages by lines limit, file size limit, or time frequency.
type FileLogWriter struct {
	*log.Logger
	FileLogConifg

	mw               *MuxWriter
	maxlinesCurlines int
	maxsizeCursize   int
	dailyOpendate    int
	startLock        sync.Mutex // Only one log can write to the file
}

// MuxWriter for an *os.File writer with locker.
type MuxWriter struct {
	sync.Mutex
	fd *os.File
}

// Write to os.File.
func (l *MuxWriter) Write(b []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	return l.fd.Write(b)
}

// SetFd set os.File in writer.
func (l *MuxWriter) SetFd(fd *os.File) {
	if l.fd != nil {
		l.fd.Close()
	}
	l.fd = fd
}

// NewFileWriter create a FileLogWriter returning as LoggerInterface.
func NewFileWriter(flag int) *FileLogWriter {
	w := &FileLogWriter{
		FileLogConifg: FileLogConifg{Filename: "",
			Maxlines: 1000000,
			Maxsize:  1 << 28, //256 MB
			Daily:    true,
			Maxdays:  7,
			Rotate:   true,
			Level:    logrus.DebugLevel},
	}
	// use MuxWriter instead direct use os.File for lock write when rotate
	w.mw = new(MuxWriter)
	// set MuxWriter as Logger's io.Writer
	w.Logger = log.New(w.mw, "", flag)
	return w
}

// Init file logger.
func (w *FileLogWriter) Init(conf FileLogConifg) (err error) {

	w.FileLogConifg = conf

	if len(w.Filename) == 0 {
		return errors.New("jsonconfig must have filename")
	}

	err = w.startLogger()
	return err
}

// start file logger. create log file and set to locker-inside file writer.
func (w *FileLogWriter) startLogger() error {
	fd, err := w.createLogFile()
	if err != nil {
		return err
	}
	w.mw.SetFd(fd)
	err = w.initFd()
	if err != nil {
		return err
	}
	return nil
}

func (w *FileLogWriter) docheck(size int) {
	w.startLock.Lock()
	defer w.startLock.Unlock()
	if w.Rotate && ((w.Maxlines > 0 && w.maxlinesCurlines >= w.Maxlines) ||
			(w.Maxsize > 0 && w.maxsizeCursize >= w.Maxsize) ||
			(w.Daily && time.Now().Day() != w.dailyOpendate)) {
		if err := w.DoRotate(); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
			return
		}
	}
	w.maxlinesCurlines++
	w.maxsizeCursize += size
}

// Write to logger
func (w *FileLogWriter) Write(p []byte) (n int, err error) {
	s := string(p)
	w.WriteMsg(s, logrus.DebugLevel)
	return len(s), nil
}

// WriteMsg write logger message into file.
func (w *FileLogWriter) WriteMsg(msg string, level logrus.Level) error {
	if level > w.Level {
		return nil
	}
	n := 24 + len(msg) // 24 stand for the length "2013/06/23 21:00:22 [T] "
	w.docheck(n)
	w.Logger.Print(msg)
	return nil
}

func (w *FileLogWriter) createLogFile() (*os.File, error) {
	// Open the log file
	fd, err := os.OpenFile(w.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
	return fd, err
}

func (w *FileLogWriter) initFd() error {
	fd := w.mw.fd
	finfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("get stat err: %s", err)
	}
	w.maxsizeCursize = int(finfo.Size())
	w.dailyOpendate = time.Now().Day()
	if finfo.Size() > 0 {
		content, err := ioutil.ReadFile(w.Filename)
		if err != nil {
			return err
		}
		w.maxlinesCurlines = len(strings.Split(string(content), "\n"))
	} else {
		w.maxlinesCurlines = 0
	}
	return nil
}

// DoRotate means it need to write file in new file.
// new file name like xx.log.2013-01-01.2
func (w *FileLogWriter) DoRotate() error {
	_, err := os.Lstat(w.Filename)
	if err == nil { // file exists
		// Find the next available number
		num := 1
		fname := ""
		for ; err == nil && num <= 999; num++ {
			fname = w.Filename + fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), num)
			_, err = os.Lstat(fname)
		}
		// return error if the last file checked still existed
		if err == nil {
			return fmt.Errorf("rotate: Cannot find free log number to rename %s", w.Filename)
		}

		// block Logger's io.Writer
		w.mw.Lock()
		defer w.mw.Unlock()

		fd := w.mw.fd
		fd.Close()

		// close fd before rename
		// Rename the file to its newfound home
		err = os.Rename(w.Filename, fname)
		if err != nil {
			return fmt.Errorf("rotate: %s", err)
		}

		// re-start logger
		err = w.startLogger()
		if err != nil {
			return fmt.Errorf("rotate StartLogger: %s", err)
		}

		go w.deleteOldLog()
	}

	return nil
}

func (w *FileLogWriter) deleteOldLog() {
	dir := filepath.Dir(w.Filename)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				returnErr = fmt.Errorf("unable to delete old log '%s', error: %+v", path, r)
			}
		}()

		if !info.IsDir() && info.ModTime().Unix() < (time.Now().Unix()-60*60*24*w.Maxdays) {
			if strings.HasPrefix(filepath.Base(path), filepath.Base(w.Filename)) {
				os.Remove(path)
			}
		}
		return
	})
}

// Destroy file logger, close file writer.
func (w *FileLogWriter) Destroy() {
	w.mw.fd.Close()
}

// Flush file logger.
// there are no buffering messages in file logger in memory.
// flush file means sync file from disk.
func (w *FileLogWriter) Flush() {
	w.mw.fd.Sync()
}
