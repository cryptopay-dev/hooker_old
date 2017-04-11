package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"archive/zip"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/xml"
)

type parser struct {
	file    os.FileInfo
	ch      chan struct{}
	options options
	prefix  string
}

func newParser(file os.FileInfo, ch chan struct{}, opts options) *parser {
	return &parser{
		ch:      ch,
		file:    file,
		options: opts,
		prefix:  file.Name(),
	}
}

func (p *parser) parse() {
	defer func() {
		p.ch <- struct{}{}
	}()
	filePath := path.Join(p.options.dir, p.file.Name())
	log.Printf("[FILE: %s] Found new file %s\n", p.prefix, filePath)

	// Checking that file have good size
	err := p.finishedUpload(filePath)
	if err != nil {
		log.Fatalf("[FILE: %s] File size checking error: %s\n", p.prefix, err)
	}

	// Sending stuff and deleting file
	buf, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("[FILE: %s] Reading file error: %s\n", p.prefix, err)
	}

	err = p.sendWithBackoff(buf, p.file.Name())
	if err != nil {
		log.Fatalf("[FILE: %s] Error sending to API: %s\n", p.prefix, err)
	}

	log.Printf("[FILE: %s] Successfully send data to API\n", p.prefix)

	// Zipping file
	if p.options.zip {
		zipname := path.Join(p.options.out, p.file.Name()+".zip")

		err := p.zipit(p.file.Name(), zipname, buf)
		if err != nil {
			log.Fatalf("[FILE: %s] Error zipping file: %s\n", p.prefix, err)
		}

		log.Printf("[FILE: %s] Zipped file to: %s\n", p.prefix, zipname)
	}

	// Deleting file
	if p.options.clear || p.options.zip {
		err = os.Remove(filePath)
		if err != nil {
			log.Fatalf("[FILE: %s] Error deleting file: %s\n", p.prefix, err)
		}

		log.Printf("[FILE: %s] Deleted file %s\n", p.prefix, filePath)
	}
}

func (p *parser) finishedUpload(filePath string) error {
	var size int64

	for {
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}

		info, err := file.Stat()
		if err != nil {
			return err
		}

		tmp := info.Size()
		if tmp == size {
			return nil
		}

		if p.options.verbose {
			log.Printf("[FILE: %s] File %s size is %d bytes\n", p.prefix, filePath, tmp)
			log.Printf("[FILE: %s] Next file size check in %d seconds\n", p.prefix, p.options.checkInterval)
		}

		size = tmp
		time.Sleep(time.Second * time.Duration(p.options.checkInterval))
	}
}

func (p *parser) sendWithBackoff(info []byte, filename string) error {
	backoff := 0

	for {
		log.Printf("[FILE: %s] Sending data to API %d try\n", p.prefix, backoff+1)

		err := p.post(info, filename)
		if err == nil {
			return nil
		}

		backoff++
		mul := math.Pow(2, float64(backoff)) // 2 4 16 32 64
		log.Printf("[FILE: %s] Error sending to API: %s\n", p.prefix, err)
		if backoff > 5 {
			break
		}

		log.Printf("[FILE: %s] Backoff for %d mins\n", p.prefix, int64(mul))
		time.Sleep(time.Minute * time.Duration(mul))
	}

	return errors.New("Unable to send data to API")
}

func (p *parser) post(data []byte, filename string) error {
	// Minification
	m := minify.New()
	m.AddFunc("xml", xml.Minify)

	minified, err := m.Bytes("xml", data)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	n, err := gz.Write(minified)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("Written 0 bytes")
	}
	gz.Close()

	req, err := http.NewRequest("POST", p.options.url, &buf)
	if req != nil {
		defer req.Body.Close()
	}

	if err != nil {
		return err
	}

	req.Header.Set("X-Access-Token", p.options.token)
	req.Header.Set("X-File-Name", filename)
	req.Header.Set("Content-Encoding", "gzip")

	tout := time.Second * time.Duration(p.options.timeout)
	transport := http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, tout)
		},
	}

	client := http.Client{
		Transport: &transport,
	}

	response, err := client.Do(req)
	if response != nil {
		defer response.Body.Close()
	}

	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Http status: %d", response.StatusCode)
	}

	return nil
}

func (p *parser) zipit(file, output string, data []byte) error {
	zipfile, err := os.Create(output)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	f, err := archive.Create(file)
	if err != nil {
		return err
	}

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	return nil
}