package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func relaySSE(w http.ResponseWriter, upstream io.Reader, transform func(eventType, data string) (string, string, bool)) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	scanner := bufio.NewScanner(upstream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var currentEvent string
	var currentData strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line = end of event
			if currentData.Len() > 0 {
				data := currentData.String()
				if transform != nil {
					evType, evData, keep := transform(currentEvent, data)
					if keep {
						if evType != "" {
							fmt.Fprintf(w, "event: %s\n", evType)
						}
						fmt.Fprintf(w, "data: %s\n\n", evData)
						flusher.Flush()
					}
				} else {
					if currentEvent != "" {
						fmt.Fprintf(w, "event: %s\n", currentEvent)
					}
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				}
			}
			currentEvent = ""
			currentData.Reset()
			continue
		}

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			if currentData.Len() > 0 {
				currentData.WriteByte('\n')
			}
			currentData.WriteString(strings.TrimPrefix(line, "data: "))
		}
	}
}

func relaySSEPassthrough(w http.ResponseWriter, upstream io.Reader) {
	relaySSE(w, upstream, nil)
}
