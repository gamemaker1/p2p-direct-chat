package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"

	"github.com/multiformats/go-multiaddr"
)

// Handle a peer connection event. Create a buffer and listen to it for read 
// events. Also write to it when input comes in.
func handleStream(stream network.Stream) {
	fmt.Println("-- Found a new stream, opening two way read-write buffer")

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	fmt.Println("-- Created buffer, now listening infinetly for reads and writes")

	go readData(rw)
	go writeData(rw)

	// The stream will stay open until you close it (or the other side closes it).
}

// Read data from the buffer connected to the other peer.
func readData(rw *bufio.ReadWriter) {
	for {
		// Don't print out empty messages
		str, _ := rw.ReadString('\n')

		if str == "" {
			return
		}
		
		if str != "\n" {
			// Peers' messages appear in green, ours in white
			fmt.Printf("\x1b[32m%s\x1b[0m$> ", str)
		}

	}
}

// Write data to the peer.
func writeData(rw *bufio.ReadWriter) {
	// Terminal input reader
	stdReader := bufio.NewReader(os.Stdin)

	// Loop forever
	for {
		// Print out a basic prompt
		fmt.Print("$> ")

		// Read input until the user hits enter
		sendData, err := stdReader.ReadString('\n')
		if err != nil {	
			fmt.Println("!! Error reading input from stdin")
			panic(err)
		}

		// Write it to the buffer
		rw.WriteString(fmt.Sprintf("%s\n", sendData))
		// Flush the buffer to ensure all data gets passed
		rw.Flush()
	}

}

func main() {
	// Define the flags for this program
	port := flag.Int("port", 0, "Port number")
	dest := flag.String("dest", "", "Destination multiaddr string")
	help := flag.Bool("help", false, "Display help")

	flag.Parse()

	if *help {
		fmt.Println("-- This program demonstrates a simple p2p chat application using libp2p\n")
		fmt.Println("-- Usage: Run './chat -port <PORT>' where <PORT> can be any port number, e.g., 6666 or 8888, etc.")
		fmt.Println("-- Now run './chat -dest <MULTIADDR>' where <MULTIADDR> is multiaddress of previous listener host.")

		os.Exit(0)
	}

	// Generate a random peer Id. This will be used to identify ourself and to 
	// generate our private key.
	peerId := rand.Reader
	fmt.Println("-- Got peer ID")

	// Creates a new RSA key pair for this host.
	privateKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, peerId)
	if err != nil {
		fmt.Println("!! Error generating RSA key pair")
		panic(err)
	}
	fmt.Println("-- Created RSA key pair")

	// Our mutli address on the IPFS protocol
	// 0.0.0.0 will listen on any interface device.
	sourceMultiAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *port))

	// Construct a new Host object that will allow us to connect to and initiate connections to other peers
	host, err := libp2p.New(
		context.Background(),
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(privateKey),
	)
	if err != nil {
		fmt.Println("!! Error creating host object")
		panic(err)
	}
	fmt.Println("-- Creating host object")

	// If the destination is not specified, then we are going to initiate the connection.
	if *dest == "" {
		// Set a function as stream handler.
		// This function is called when a peer connects, and starts a stream with this protocol.
		// Only applies on the receiving side.
		host.SetStreamHandler("/chat/1.0.0", handleStream)

		// Let's get the actual TCP port from our listen multiaddr, in case we're using 0 (default; random available port).
		var port string
		for _, la := range host.Network().ListenAddresses() {
			if p, err := la.ValueForProtocol(multiaddr.P_TCP); err == nil {
				port = p
				break
			}
		}

		if port == "" {
			fmt.Printf("!! Unable to find local port %s\n", port)
			panic("Unable to find actual local port")
		}

		// Wait for a connection.
		fmt.Printf("-- This node's multiaddr is /ip4/127.0.0.1/tcp/%v/p2p/%s. To connect to it, run another node and specify this address with the dest option.\n", port, host.ID().Pretty())

		// Hang forever. When the other peer tries to make a connection, then the 
		// handleStream function will take over.
		<-make(chan struct{})
	} else {
		fmt.Println("-- This node's multiaddrs are:")
		for _, la := range host.Addrs() {
			fmt.Printf(" - %v\n", la)
		}
		fmt.Println()

		// Turn the destination into a multiaddr.
		maddr, err := multiaddr.NewMultiaddr(*dest)
		if err != nil {
			fmt.Println("!! Failed to parse multiaddr provided.")
			panic(err)
		}
		fmt.Println("-- Parsing multiaddr")

		// Extract the peer ID from the multiaddr.
		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			fmt.Println("!! Failed to get the peer's ID from multiaddr provided.")
			panic(err)
		}
		fmt.Println("-- Retrieving peer ID")

		// Add the destination's peer multiaddress in the peerstore.
		// This will be used during connection and stream creation by libp2p.
		host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)

		// Start a stream with the destination.
		// Multiaddress of the destination peer is fetched from the peerstore using 'peerId'.
		s, err := host.NewStream(context.Background(), info.ID, "/chat/1.0.0")
		if err != nil {
			fmt.Println("!! Failed to initiate stream with host")
			panic(err)
		}
		fmt.Println("-- Initiated stream with host.")

		// Create a buffered stream so that read and writes are non blocking.
		rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

		// Create a thread to read and write data.
		go writeData(rw)
		go readData(rw)

		// Hang forever.
		select {}
	}
}
