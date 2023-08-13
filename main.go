//
// Copyright (c) 2023 Christian Pointner <equinox@spreadspace.org>
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// * Redistributions of source code must retain the above copyright notice, this
//   list of conditions and the following disclaimer.
//
// * Redistributions in binary form must reproduce the above copyright notice,
//   this list of conditions and the following disclaimer in the documentation
//   and/or other materials provided with the distribution.
//
// * Neither the name of whawty.auth nor the names of its
//   contributors may be used to endorse or promote products derived from
//   this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	nflog "github.com/florianl/go-nflog/v2"
	"golang.org/x/net/ipv4"
)

func formatIPv4(packet []byte) (msg string) {

	header, err := ipv4.ParseHeader(packet)
	if err != nil {
		return " <invalid not IPv4??>"
	}

	msg = msg + fmt.Sprintf(" LEN=%d", header.TotalLen)
	msg = msg + " SRC=" + header.Src.String()
	msg = msg + " DST=" + header.Dst.String()

	msg = msg + " PROTO=" + ipProtoToString(header.Protocol)
	// TODO: parse:
	//   SPT=     source port
	//   DPT=     destinaton port

	return msg
}

func format(attrs nflog.Attribute) (msg string) {
	if attrs.Prefix != nil {
		msg = *(attrs.Prefix)
	}
	if attrs.InDev != nil {
		intf, err := net.InterfaceByIndex(int(*attrs.InDev))
		if err == nil {
			msg = msg + " IN=" + intf.Name
		} else {
			msg = msg + " IN=<invalid>"
		}
	} else {
		msg = msg + " IN="
	}
	if attrs.OutDev != nil {
		intf, err := net.InterfaceByIndex(int(*attrs.OutDev))
		if err == nil {
			msg = msg + " OUT=" + intf.Name
		} else {
			msg = msg + " OUT=<invalid>"
		}
	} else {
		msg = msg + " OUT="
	}

	if attrs.HwType != nil {
		msg = msg + " HWTYPE=" + hwTypeToString(*attrs.HwType)
		if attrs.HwProtocol != nil {
			msg = msg + " HWPROTO=" + hwProtocolToString(*attrs.HwType, *attrs.HwProtocol)
		} else {
			msg = msg + " HWPROTO="
		}
	} else {
		msg = msg + " HWTYPE="
	}

	if attrs.Payload != nil {
		// TODO: for now we assume IPv4
		msg = msg + formatIPv4(*attrs.Payload)
	}

	return msg
}

func main() {
	nflogGroup := flag.Uint("group", 0, "NFLOG Group ID to listen to")
	flag.Parse()

	if *nflogGroup > uint(^uint16(0)) {
		fmt.Fprintf(os.Stderr, "invalid group id %d (must be between 0 and %d)\n", *nflogGroup, ^uint16(0))
		os.Exit(1)
	}

	config := nflog.Config{
		Group:    uint16(*nflogGroup),
		Copymode: nflog.CopyPacket,
	}

	nf, err := nflog.Open(&config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not open nflog socket: %v\n", err)
		os.Exit(2)
	}
	defer nf.Close()
	fmt.Fprintf(os.Stderr, "listening to NFLOG Group %d\n", config.Group)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hook := func(attrs nflog.Attribute) int {
		fmt.Fprintf(os.Stdout, "%s  %s\n", attrs.Timestamp.Format("2006-01-02 15:04:05"), format(attrs))
		return 0
	}

	errFunc := func(err error) int {
		fmt.Fprintf(os.Stderr, "received error on hook: %v\n", err)
		return 0
	}

	err = nf.RegisterWithErrorFunc(ctx, hook, errFunc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to register hook function: %v\n", err)
		os.Exit(2)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}
