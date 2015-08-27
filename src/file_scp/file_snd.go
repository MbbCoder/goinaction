package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"
)

const (
	VERSION    = "1.0.0"
	REQ_HEADER = "10001" // 0X2711
)

func printVersion() {
	fmt.Println("file snd" + VERSION + " by gerryyang")
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal error: %s\n", err.Error())
		os.Exit(1)
	}
}

func handleCommandLine() (file string, job int64, service string) {
	var f = flag.String("f", "data.txt", "file")
	var j = flag.Int64("j", 1, "job")
	var s = flag.String("s", "127.0.0.1:9001", "host:ip")
	var printVer bool
	flag.BoolVar(&printVer, "v", false, "print version")

	flag.Parse()
	if printVer {
		printVersion()
		os.Exit(0)
	}
	fmt.Println("file:", *f)
	fmt.Println("job:", *j)
	fmt.Println("host:ip:", *s)
	return *f, *j, *s
}

func Int64ToBytes(i int64) []byte {
	var buf = make([]byte, 8) // int64 is 8 byte
	binary.LittleEndian.PutUint64(buf, uint64(i))
	return buf
}

func BytesToInt64(buf []byte) int64 {
	return int64(binary.LittleEndian.Uint64(buf))
}

func proc(req *string, cid int, offset int64, service string, lockChan chan bool) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	checkError(err)

	var cid_name string
	cid_name = fmt.Sprintf("%d", cid)

	send(tcpAddr, req, cid_name, offset, lockChan)

}

func send(tcpAddr *net.TCPAddr, req *string, cid_name string, offset int64, lockChan chan bool) {
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DialTCP: cid_name[%s] err[%s]\n", cid_name, err.Error())
		lockChan <- true
		return
	}

	var cid int
	fmt.Sscanf(cid_name, "%d", &cid)

	req_header, _ := strconv.ParseInt(REQ_HEADER, 10, 64)

	var bytes_buf bytes.Buffer
	bytes_buf.Write(Int64ToBytes(req_header))
	bytes_buf.Write(Int64ToBytes(offset))
	bytes_buf.Write([]byte(*req))

	fmt.Printf("send: cid_name[%s] bytes_buf[%#v]\n", cid_name, bytes_buf.Bytes())
	//fmt.Printf("%q\n", bytes_buf.Bytes())

	_, err = conn.Write(bytes_buf.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Write: cid_name[%s] err[%s]\n", cid_name, err.Error())
		lockChan <- true
		return
	}

	var ans_buf [4]byte
	_, err = conn.Read(ans_buf[0:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Read: cid_name[%s] err[%s]\n", cid_name, err.Error())
		lockChan <- true
		return
	}
	fmt.Fprintf(os.Stdout, "Read: cid_name[%s] ans_buf[%s]\n", cid_name, ans_buf)

	lockChan <- true
	conn.Close()
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	file, job, service := handleCommandLine()

	fin, err := os.Open(file)
	checkError(err)
	defer fin.Close()

	var offset int64 = 0
	buf := make([]byte, job)

	fin_info, _ := fin.Stat()
	lockChanNum := fin_info.Size()/job + 1
	lockChan := make(chan bool, lockChanNum)

	start := time.Now()
	var cid int = 0

	for {
		cnt, err := fin.ReadAt(buf, offset)
		if err == io.EOF {
			if cnt == 0 {
				break
			}
			fmt.Printf("cid[%d] offset[%d] read bytes[%d] buf[%q]\n", cid, offset, cnt, buf[:cnt])
			var req string = string(buf[:cnt])
			go proc(&req, cid, offset, service, lockChan)
			offset += int64(cnt)
			cid++
			break

		} else if cnt != len(buf) {
			continue

		} else {
			fmt.Printf("cid[%d] offset[%d] read bytes[%d] buf[%q]\n", cid, offset, cnt, buf[:cnt])
			var req string = string(buf[:cnt])
			go proc(&req, cid, offset, service, lockChan)
			offset += int64(len(buf))
			cid++
		}
	}

	for i := 0; i < int(cid-1); i++ {
		<-lockChan
	}

	elapsed := 1000000 * time.Since(start).Seconds()
	fmt.Println("time elapsed: ", elapsed, "us")
}