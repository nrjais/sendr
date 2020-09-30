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

func server() {
	var candidatesMux sync.Mutex
	pendingCandidates := make([]*webrtc.ICECandidate, 0)
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

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
		} else if onICECandidateErr := signalCandidate("rec", c); onICECandidateErr != nil {
			panic(onICECandidateErr)
		}
	})

	// A HTTP handler that allows the other Pion instance to send us ICE candidates
	// This allows us to add ICE candidates faster, we don't have to wait for STUN or TURN
	// candidates which may be slower
	go func() {
		for {
			res, err := http.Get("http://localhost:8080/candidate/sen")
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
			res, err := http.Get("http://localhost:8080/sdp/sen")
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

			// Create an answer to send to the other process
			answer, err := peerConnection.CreateAnswer(nil)
			checkErr(err)

			// Send our answer to the HTTP server listening in the other process
			payload, err := json.Marshal(answer)
			checkErr(err)
			resp, err := http.Post("http://localhost:8080/sdp/rec", "application/json; charset=utf-8", bytes.NewReader(payload))
			checkErr(err)
			resp.Body.Close()

			// Sets the LocalDescription, and starts our UDP listeners
			err = peerConnection.SetLocalDescription(answer)
			checkErr(err)

			candidatesMux.Lock()
			for _, c := range pendingCandidates {
				err := signalCandidate("rec", c)
				checkErr(err)
			}
			candidatesMux.Unlock()
		}
	}()

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		// Register channel opening handling
		d.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())

			for range time.NewTicker(5 * time.Second).C {
				message := "123456789012345"
				fmt.Printf("Sending '%s'\n", message)

				// Send the message as text
				sendTextErr := d.SendText(message)
				if sendTextErr != nil {
					panic(sendTextErr)
				}
			}
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label(), string(msg.Data))
		})
	})
	select{}
}
