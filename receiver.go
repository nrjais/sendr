package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
)

func client() {
	var candidatesMux sync.Mutex
	pendingCandidates := make([]*webrtc.ICECandidate, 0)

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// When an ICE candidate is available send to the other Pion instance
	// the other Pion instance will add this candidate by calling AddICECandidate
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		desc := peerConnection.RemoteDescription()
		if desc == nil {
			pendingCandidates = append(pendingCandidates, c)
		} else if onICECandidateErr := signalCandidate("sen", c); err != nil {
			panic(onICECandidateErr)
		}
	})

	// A HTTP handler that allows the other Pion instance to send us ICE candidates
	// This allows us to add ICE candidates faster, we don't have to wait for STUN or TURN
	// candidates which may be slower
	go func() {
		for {
			res, err := http.Get("http://localhost:8080/candidate/rec")
			checkErr(err)
			if res.StatusCode != 200 {
				time.After(time.Second * 1)
				continue
			}
			defer res.Body.Close()
			candidate, err := ioutil.ReadAll(res.Body)
			checkErr(err)
			err = peerConnection.AddICECandidate(webrtc.ICECandidateInit{Candidate: string(candidate)})
			checkErr(err)

			time.After(time.Second * 1)
		}
	}()

	// A HTTP handler that processes a SessionDescription given to us from the other Pion process
	go func() {
		for {
			res, err := http.Get("http://localhost:8080/sdp/rec")
			checkErr(err)
			if res.StatusCode != 200 {
				time.After(time.Second * 1)
				continue
			}
			sdp := webrtc.SessionDescription{}
			err = json.NewDecoder(res.Body).Decode(&sdp)
			checkErr(err)

			err = peerConnection.SetRemoteDescription(sdp)
			checkErr(err)

			candidatesMux.Lock()
			defer candidatesMux.Unlock()

			for _, c := range pendingCandidates {
				err = signalCandidate("sen", c)
				checkErr(err)
			}
		}
	}()
	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label(), dataChannel.ID())

		for range time.NewTicker(5 * time.Second).C {
			message := "123456789012345"
			fmt.Printf("Sending '%s'\n", message)

			// Send the message as text
			sendTextErr := dataChannel.SendText(message)
			if sendTextErr != nil {
				panic(sendTextErr)
			}
		}
	})

	// Register text message handling
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
	})

	// Create an offer to send to the other process
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Send our offer to the HTTP server listening in the other process
	payload, err := json.Marshal(offer)
	if err != nil {
		panic(err)
	}
	resp, err := http.Post("http://localhost:8080/sdp/sen", "application/json; charset=utf-8", bytes.NewReader(payload))
	if err != nil {
		panic(err)
	} else if err := resp.Body.Close(); err != nil {
		panic(err)
	}

	// Block forever
	select {}
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
