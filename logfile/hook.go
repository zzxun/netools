package logfile

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/heralight/logrus_mate"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus_mate.RegisterHook("dailyfile", NewFileHook)
}

var (
	fileNameFill []interface{}
)

// NewFileHook return a new logrus Hook
func NewFileHook(options logrus_mate.Options) (hook logrus.Hook, err error) {

	conf := FileLogConifg{}

	if err = options.ToObject(&conf); err != nil {
		return
	}

	path := strings.Split(conf.Filename, "/")
	if len(path) > 1 {
		exec.Command("mkdir", path[0]).Run()
	}

	conf.Filename = fmt.Sprintf(conf.Filename, fileNameFill...)

	w := NewFileWriter(log.Ldate | log.Ltime)

	if err = w.Init(conf); err != nil {
		return
	}

	w.SetPrefix("[Reconci] ")

	hook = &FileHook{W: w}

	return
}

// FileHook wrapper file writer implement logrus.Hook
type FileHook struct {
	W *FileLogWriter
}

// Fire implement logrus.Hook#Fire.
func (p *FileHook) Fire(entry *logrus.Entry) (err error) {
	message, err := getMessage(entry)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read entry, %v", err)
		return err
	}
	switch entry.Level {
	case logrus.PanicLevel:
		fallthrough
	case logrus.FatalLevel:
		fallthrough
	case logrus.ErrorLevel:
		return p.W.WriteMsg(fmt.Sprintf("[ERROR] %s", message), logrus.ErrorLevel)
	case logrus.WarnLevel:
		return p.W.WriteMsg(fmt.Sprintf("[WARN] %s", message), logrus.WarnLevel)
	case logrus.InfoLevel:
		return p.W.WriteMsg(fmt.Sprintf("[INFO] %s", message), logrus.InfoLevel)
	case logrus.DebugLevel:
		return p.W.WriteMsg(fmt.Sprintf("[DEBUG] %s", message), logrus.DebugLevel)
	default:
	}

	return nil
}

// Levels implement logrus.Hook#Levels.
func (p *FileHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

func getMessage(entry *logrus.Entry) (message string, err error) {
	message = message + fmt.Sprintf("%s\n", entry.Message)
	for k, v := range entry.Data {
		if !strings.HasPrefix(k, "err_") {
			message = message + fmt.Sprintf("%v:%v\n", k, v)
		}
	}
	if errCode, exist := entry.Data["err_code"]; exist {

		ns, _ := entry.Data["err_ns"]
		ctx, _ := entry.Data["err_ctx"]
		id, _ := entry.Data["err_id"]
		tSt, _ := entry.Data["err_stack"]
		st, _ := tSt.(string)
		st = strings.Replace(st, "\n", "\n\t\t", -1)

		buf := bytes.NewBuffer(nil)
		buf.WriteString(fmt.Sprintf("\tid:\n\t\t%s#%d:%s\n", ns, errCode, id))
		buf.WriteString(fmt.Sprintf("\tcontext:\n\t\t%s\n", ctx))
		buf.WriteString(fmt.Sprintf("\tstacktrace:\n\t\t%s", st))

		message = message + fmt.Sprintf("%v", buf.String())
	}

	return
}
