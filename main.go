package main

import (
	"flag"
	"fmt"
	"github.com/rwcarlsen/goexif/exif"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var filesByDate = make(map[Date]FileDetails)
var otherFilePaths []string

type Date struct {
	year int
	month int
	day int
}

func (d Date) String() string {
	return fmt.Sprintf("%v-%v-%v", d.year, d.month, d.day)
}

type FileDetails []*FileDetail

type FileDetail struct {
	p string
	err error
	e string
	d Date
}

func (d *FileDetails) String() string {
	s := ""
	for _, detail := range *d {
		s += fmt.Sprintf("Path: %s, Date: %s | ", detail.p, detail.d)
	}
	return s
}

func main() {
	s := flag.String("s", "./source", "source directory")
	d := flag.String("d", "./dest", "destination directory")
	o := flag.String("o", "./others", "files that has no date")
	flag.Parse()

	path := filepath.FromSlash(*s)
	dest := filepath.FromSlash(*d)
	other := filepath.FromSlash(*o)

	dir, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	readImages(dir, path)
	lenOfFiles := 0
	for _, files := range filesByDate {
		lenOfFiles += len(files)
	}
	fmt.Printf("Reading files completed. Number of files: %v, Number of other files: %v\n", lenOfFiles, len(otherFilePaths))
	log.Printf("Starting to sort files.\n")
	p := 0
	var dp string
	m := sync.Mutex{}
	go func() {
		for {
			m.Lock()
			log.Printf("Processing: %v / %v | Current: %s\n", p, lenOfFiles, dp)
			m.Unlock()
			time.Sleep(time.Second * 5)
		}
	}()
	for date, details := range filesByDate {
		for i, detail := range details {
			if detail.err != nil {
				dst := fmt.Sprintf(dest + string(os.PathSeparator) + "err" + string(os.PathSeparator) + date.String() + "-" + strconv.Itoa(i + 1) + "." + detail.e)
				_, err := Copy(detail.p, dst)
				if err != nil {
					log.Printf("Error reading file. Path: %s, Err: %s\n", detail.p, err)
				}
			} else {
				dst := fmt.Sprintf(dest + string(os.PathSeparator) + strconv.Itoa(date.year) + string(os.PathSeparator) + strconv.Itoa(date.month) + string(os.PathSeparator) + strconv.Itoa(date.day) + string(os.PathSeparator) + date.String() + "-" + strconv.Itoa(i + 1) + "." + detail.e)
				_, err := Copy(detail.p, dst)
				if err != nil {
					log.Printf("Error reading file. Path: %s, Err: %s\n", detail.p, err)
				}
			}
			m.Lock()
			dp = detail.p
			p++
			m.Unlock()
		}
	}
	log.Printf("Processing: %v / %v | Current: %s\n", p, lenOfFiles, dp)

	log.Printf("Start to copy other files:")
	p = 0
	dp = ""
	for _, file := range otherFilePaths {
		_, f := filepath.Split(file)
		dst := fmt.Sprintf(other + string(os.PathSeparator) + f)
		_, err := Copy(file, dst)
		if err != nil {
			log.Printf("Error reading file. Path: %s, Err: %s\n", file, err)
		}
		m.Lock()
		dp = file
		p++
		m.Unlock()
	}
	log.Printf("Sorting finished.\n")
}

func Copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("REGULAR %s", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("OPEN %s", src)
	}
	defer source.Close()

	dir, _ := filepath.Split(dst)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return 0, fmt.Errorf("CREATE DIR %s", dir)
		}
	}
	destination, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("CREATE %s", dst)
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func readImages(dir []os.DirEntry, s string) {
	for _, entry := range dir {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := s + string(os.PathSeparator) + info.Name()
		//ext := strings.ToLower(filepath.Ext(path))
		if info.IsDir() {
			subDir, err := os.ReadDir(path)
			if err != nil {
				log.Fatal(err)
			}
			readImages(subDir, path)
		} else /*if strings.Contains(ext, "jpg") || strings.Contains(ext, "jpeg") || strings.Contains(ext, "png")*/ {
			d, err := readDate(path)
			if err != nil {
				otherFilePaths = append(otherFilePaths, path)
				log.Printf("Error while reading date. File: %s, Err: %s", path, err)
			}
			filesByDate[d] = append(filesByDate[d], &FileDetail{
				p: path,
				err: err,
				e: filepath.Ext(path),
				d: d,
			})
		}
	}
}

func readDate(p string) (Date, error) {
	file, err := os.Open(p)
	if err != nil {
		year, month, day, err := readMod(p)
		if err != nil {
			log.Printf("Error reading file. Path: %s, Err: %s", p, err)
			return Date{}, err
		}
		return Date{
			year:  year,
			month: month,
			day:   day,
		}, nil
	}
	decode, err := exif.Decode(file)
	if err != nil {
		year, month, day, err := readMod(p)
		if err != nil {
			log.Printf("Error reading metainfo. Path: %s, Err: %s", p, err)
			return Date{}, err
		}
		return Date{
			year:  year,
			month: month,
			day:   day,
		}, nil
	}
	dateTime, err := decode.DateTime()
	if err != nil {
		year, month, day, err := readMod(p)
		if err != nil {
			log.Printf("Error reading dateTime. Path: %s, Err: %s", p, err)
			return Date{}, err
		}
		return Date{
			year:  year,
			month: month,
			day:   day,
		}, nil
	}
	year, month, day := dateTime.Date()
	return Date{
		year:  year,
		month: int(month),
		day:   day,
	}, nil
}

func readMod(p string) (year, month, day int, err error) {
	stat, err := os.Stat(p)
	if err != nil {
		return 0, 0, 0, err
	}
	y, m, d := stat.ModTime().Date()
	return y, int(m), d, nil

}
