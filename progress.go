package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
)

// 782448  63%  110.64kB/s    0:00:04

type Progress struct {
	r           io.Reader
	w           io.Writer
	size        int64
	begin       time.Time
	lastPrinted time.Time
	count       int64
	onceBegin   sync.Once
	stop        chan struct{}
	onceStop    sync.Once
}

func newReadProgress(r io.Reader, size int64) *Progress {
	return &Progress{
		r:     r,
		size:  size,
		begin: time.Now(),
		stop:  make(chan struct{}),
	}
}

func newWriteProgress(w io.Writer, size int64) *Progress {
	return &Progress{
		w:     w,
		size:  size,
		begin: time.Now(),
		stop:  make(chan struct{}),
	}
}

func (p *Progress) Close() {
	p.onceStop.Do(func() { close(p.stop) })
}

func (p *Progress) Read(buf []byte) (int, error) {
	return p.rwFunc(p.r.Read, buf)
}

func (p *Progress) Write(buf []byte) (int, error) {
	return p.rwFunc(p.w.Write, buf)
}

func (p *Progress) rwFunc(f func([]byte) (int, error), buf []byte) (int, error) {
	// start printing on first call
	p.onceBegin.Do(func() { go p.run() })

	n, err := f(buf)
	atomic.AddInt64(&p.count, int64(n))
	return n, err
}

func (p *Progress) run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.maybePrintSpeed()
		case <-p.stop:
			return
		}
	}
}

func (p *Progress) maybePrintSpeed() {
	now := time.Now()
	elapsed := now.Sub(p.lastPrinted)
	if elapsed >= time.Second {
		p.printSpeed(now)
		p.lastPrinted = now
	}
}

func (p *Progress) printSpeed(now time.Time) {
	count := atomic.LoadInt64(&p.count)

	percent := "?"
	if p.size > 0 {
		percent = strconv.FormatInt(count*100/p.size, 10)
	}

	speed := "?"
	elapsedTime := now.Sub(p.begin)
	elapsedSeconds := elapsedTime.Seconds()
	if elapsedSeconds > 0 {
		bytesPerSecond := float64(count) / elapsedTime.Seconds()
		speed = strings.Replace(humanize.Bytes(uint64(bytesPerSecond)), " ", "", -1)
	}

	remainingTimeString := "?s"
	if p.size > 0 {
		totalTime := time.Duration(float64(elapsedTime) * (float64(p.size) / float64(count)))
		remainingTime := totalTime - elapsedTime
		if remainingTime < 0 {
			remainingTime = 0
		}
		remainingTimeString = remainingTime.String()
	}

	fmt.Fprintf(os.Stderr, "%s %s%% %s/s %s\n", humanize.Comma(count), percent, speed, remainingTimeString)
}
