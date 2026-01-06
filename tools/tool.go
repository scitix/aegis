package tools

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
	"text/template"
	"time"

	"k8s.io/klog/v2"
)

func init() {
	timeLocal := time.FixedZone("CST", 3600*8)
	time.Local = timeLocal
}

func RandomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func CompressTargz(input, output string) error {
	var file *os.File
	var err error
	var writer *gzip.Writer
	var body []byte

	if file, err = os.OpenFile(output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644); err != nil {
		return err
	}
	defer file.Close()

	if writer, err = gzip.NewWriterLevel(file, gzip.BestCompression); err != nil {
		return err
	}
	defer writer.Close()

	tw := tar.NewWriter(writer)
	defer tw.Close()

	if body, err = ioutil.ReadFile(input); err != nil {
		return err
	}

	if body != nil {
		hdr := &tar.Header{
			Name: path.Base(input),
			Mode: int64(0o644),
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
	}
	if _, err := tw.Write(body); err != nil {
		return err
	}
	return nil
}

func RenderWorkflowTemplate(tmp string, parameters map[string]interface{}) (string, error) {
	keep := func(key string) string {
		return fmt.Sprintf("{{.%s}}", key)
	}
	funcMap := template.FuncMap{
		"keep": keep,
	}

	tmpl := template.Must(template.New("workflow.tmp").Funcs(funcMap).Parse(tmp))
	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, parameters)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func LoadFromFile(file string) (string, error) {
	bytes, err := ioutil.ReadFile(file)
	return string(bytes), err
}

func GetCurrentTimestamp() string {
	return time.Now().Format("2006.01.02")
}

func GetTimestampFromUnix(unix int64) string {
	return time.Unix(unix, 0).Format("2006.01.02 15:04:05")
}

func GetCurrentTimestampToSecond() string {
	return time.Now().Format("2006.01.02 15:04:05")
}

func ParseTimestamps(value string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", value)
}

type Exiter func(code int)

func HandlerSigterm(cancel context.CancelFunc, delay int, exit Exiter) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	klog.InfoS("Received SIGTERM, shutting down")

	klog.Infof("Handled quit, delaying controller exit for %d seconds", delay)
	time.Sleep(time.Duration(delay) * time.Second)
	cancel()
	time.Sleep(time.Duration(delay) * time.Second)

	exit(0)
}

func GetKubeConfigPath() string {
	if file := os.Getenv("KUBECONFIG"); file != "" {
		return file
	}

	return filepath.Join(os.Getenv("HOME"), ".kube", "config")
}
