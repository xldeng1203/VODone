package main

import (
	"bufio"
	"sync"

	"VODone/Client/msgs"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/zhuangsirui/binpacker"
	"io"
	"net"
	"time"
)

var readerLogin *bufio.Reader
var writerLogin *bufio.Writer
var writeLockLogin sync.RWMutex
var MsgChanLogin chan *msgs.Message
var ExitChanLogin chan int
var PoolLogin sync.Pool

func startLoginServerLoop(conn net.Conn) {
	fmt.Printf("LoginServer start goroutine\n")
	if _, err := Send2Login(conn, []byte("  V1")); err != nil {
		fmt.Printf("send protocol err\n")
		panic(err)
	}

	wg.Wrap(func() {
		client2LoginServerLoop(conn)
	})
}

func client2LoginServerLoop(client net.Conn) {
	fmt.Printf("client2LoginServerLoop remoteAddr[%v] localAddr[%v]\n", client.RemoteAddr(), client.LocalAddr())
	var err error
	var header byte
	var cmd uint32
	var length uint32

	msgPumpStartedChan := make(chan bool)
	go clientMsgPumpLogin(client, msgPumpStartedChan)
	<-msgPumpStartedChan

	buf := make([]byte, ProtocolHeaderLen)
	for {
		_, err = io.ReadFull(readerLogin, buf)
		if err != nil {
			fmt.Printf("client2LoginServerLoop read head from remote[%v] err->%v buffed->%v\n", client.RemoteAddr(), err, readerLogin.Buffered())
			//ExitChanLogin <- 1
			break
		}

		// header
		header = buf[0]
		if header != 0x05 {
			err = fmt.Errorf("client2LoginServerLoop header[%s] err", header)
			//ExitChanLogin <- 1
			break
		}

		// cmd
		cmd = binary.BigEndian.Uint32(buf[1:5])

		// length
		length = binary.BigEndian.Uint32(buf[5:9])

		// data
		data := make([]byte, length)
		_, err = io.ReadFull(readerLogin, data)
		if err != nil {
			fmt.Printf("client2LoginServerLoop read data from client[%v] err->%v buffed->%v", client.RemoteAddr(), err, readerLogin.Buffered())
			//ExitChanLogin <- 1
			break
		}

		fmt.Printf("client2LoginServerLoop header[%v] cmd[%v] len[%d] data[%x]\n", header, cmd, length, data)

		// new msg
		//msg := Pool.Get().(*msgs.Message)
		//msg := &msgs.Message{ID:(int32)(cmd),Body:data,Conn:client}
		var msg msgs.Message
		msg.ID = int(cmd)
		msg.Body = data
		msg.Len = (int)(length)
		msg.Conn = client

		MsgChanLogin <- &msg
	}

	client.Close()
	//ExitChanLogin <- 1

	defer func() {
		fmt.Printf("client2LoginServerLoop exit\n")
	}()
}

func clientMsgPumpLogin(client net.Conn, startedChan chan bool) {
	close(startedChan)

	hbTickerLogin := time.NewTicker(C2LoginServerHB)
	hbChanLogin := hbTickerLogin.C
	for {
		select {
		case <-hbChanLogin:
			buf := new(bytes.Buffer)
			packer := binpacker.NewPacker(buf, binary.BigEndian)
			packer.PushByte(0x05)
			packer.PushInt32(10010)
			packer.PushInt32(0)
			if err := packer.Error(); err != nil {
				fmt.Printf("clientMsgPumpLogin make msg err [%v]\n", err)
				ExitChanLogin <- 1
			}

			fmt.Printf("clientMsgPumpLogin heartbeat buf[%x] \n", buf.Bytes())

			if _, err := Send2Login(client, buf.Bytes()); err != nil {
				fmt.Printf("clientMsgPumpLogin send heartbeat packet err[%v] \n", err)
				ExitChanLogin <- 1
			}
		case msg, ok := <-MsgChanLogin:
			if ok {
				fmt.Printf("clientMsgPumpLogin msgChan msg[%v] body[%v]\n", msg.ID, msg.Body)
				if msg.ID == 10014 {
					buf := new(bytes.Buffer)
					packer := binpacker.NewPacker(buf, binary.BigEndian)
					packer.PushString(string(msg.Body[:]))
					unpacker := binpacker.NewUnpacker(buf, binary.BigEndian)

					var flag byte
					if err := unpacker.FetchByte(&flag).Error(); err != nil {
						fmt.Printf("clientMsgPumpLogin unpacker err[%v]\n", err)
						ExitChanLogin <- 1
					}

					fmt.Printf("clientMsgPumpLogin flag[%v]\n", flag)
					if flag == 48 {
						//todo
						// login server return err, and connect to queue server
						var addr string
						len := uint64(msg.Len - 1)
						if err := unpacker.FetchString(len, &addr).Error(); err != nil {
							fmt.Printf("clientMsgPumpLogin ogin failed and get queue server addr err\n")
						}
						fmt.Printf("clientMsgPumpLogin login failed and redirect to queue server[%v]\n", addr)
						connect2QueueServer(addr)
						ExitChanLogin <- 1
					} else {
						fmt.Printf("clientMsgPumpLogin login success\n")
					}
				}
			} else {
				fmt.Printf("clientMsgPumpLogin from MsgChan not ok\n")
				ExitChanLogin <- 1
			}
		case <-ExitChanLogin:
			fmt.Printf("clientMsgPumpLogin exitChan recv EXIT\n")
			goto exit
		}
	}

exit:
	client.Close()
	hbTickerLogin.Stop()
	close(ExitChanLogin)

	defer func() {
		fmt.Printf("clientMsgPumpLogin exit\n")
	}()
}

func Send2Login(c net.Conn, data []byte) (int, error) {
	writeLockLogin.Lock()
	// todo

	// check write len(data) size buf
	n, err := writerLogin.Write(data)
	if err != nil {
		writeLockLogin.Unlock()
		return n, err
	}
	writerLogin.Flush()
	writeLockLogin.Unlock()

	return n, nil
}

func sendLoginPakcet(conn net.Conn, interVal time.Duration) {
	// todo
	// 向LoginServer发送登录信息
	ticker := time.NewTicker(time.Second * interVal)
	for _ = range ticker.C {
		buf := new(bytes.Buffer)
		packer := binpacker.NewPacker(buf, binary.BigEndian)
		packer.PushByte(0x05)
		packer.PushInt32(10013)
		accout := "account"
		passwd := "passwd"
		len := len(accout) + len(passwd)
		packer.PushInt32((int32)(len))
		packer.PushString(accout).PushString(passwd)
		if err := packer.Error(); err != nil {
			fmt.Printf("make msg err [%v]\n", err)
			panic(err)
		}

		fmt.Printf("client send c2slogin packet buf[%x] dataLen[%v]\n", buf.Bytes(), len)

		if _, err := Send2Login(conn, buf.Bytes()); err != nil {
			fmt.Printf("send c2slogin packet err[%v] \n", err)
			panic(err)
		}

		ticker.Stop()
	}
}

func connect2LoginServer(addr string) net.Conn {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}

	readerLogin = bufio.NewReaderSize(conn, defaultBufferSize)
	writerLogin = bufio.NewWriterSize(conn, defaultBufferSize)

	startLoginServerLoop(conn)
	return conn
}
