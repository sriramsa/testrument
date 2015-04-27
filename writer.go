// Package instrument provides EventWriter, the main object that the clients
// use to instrument.
// Attachingn to the stream will provide a stream of JSON strings delimited with
// newline.
package testrument

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// Object that provides interfaces to write an event
type EventStream struct {
	name   string // Package name
	node   string // Node name
	pretty bool   // Should this be pretty printed

	out io.Writer // MultiWriter, used for writing

	swapOut    io.Writer   // New MultiWriter to be used
	logReaders []io.Writer // List of writers attached. Used for attach/detach op
	log        *log.Logger // Log package logger to log events
}

// Create a new event writer
// TODO: Move 'pretty' to attach function, doesn't make sense here.
func NewEventStream(name string, pretty bool) *EventStream {
	h, err := os.Hostname()
	if err != nil {
		log.Println("error getting hostname : ", err)
		h = "<ERROR-READING>"
	}

	w := &EventStream{
		name:   name,
		node:   h,
		pretty: pretty,
	}

	return w
}

func (w *EventStream) attach(out io.Writer) error {

	// If this is the first attatch
	if w.logReaders == nil {
		// Create the channel
		w.logReaders = make([]io.Writer, 1)
		w.logReaders[0] = out

		// Create a pipe to let Log package write to, for us
		// to listen to
		pr, pw := io.Pipe()

		// Create a log instance from the Log package
		w.log = log.New(pw, "", 0)
		w.out = io.MultiWriter(w.logReaders...)

		// Start a co-routine to write in the background
		go w.writer(pr)
	} else {
		w.logReaders = append(w.logReaders, out)
		w.swapOut = io.MultiWriter(w.logReaders...)
	}

	return nil
}

func (w *EventStream) detach(out io.Writer) {
	// If there is only one reader attached,
	if len(w.logReaders) == 1 {
		w.log = nil

		// Convert to pipe writer to get to the close interface
		var pr *io.PipeWriter
		pr = out.(*io.PipeWriter)
		if pr != nil {
			log.Println("Closing Pipe")
			pr.Close()
		} else {
			log.Println("Detach failed")
		}

		w.logReaders = nil

		// This will release the writer and let the
		w.out = nil

		log.Println("Detached : ", out)
		// signal writer to exit
		return
	}

	// If there are more than one reader attached
	for i := range w.logReaders {
		if w.logReaders[i] == out {

			newList := make([]io.Writer, 0)
			// If first element
			if i != 0 {
				newList = append(newList, w.logReaders[:i]...)
			}
			// If last element
			if i+1 != len(w.logReaders) {
				newList = append(newList, w.logReaders[i+1:]...)
			}
			w.logReaders = newList

			w.swapOut = io.MultiWriter(newList...)
		}
	}
	log.Println("Detached : ", out)
}

// Write to all the writers attached.
// Swap out the writer to new writer list if there is a new list
func (w *EventStream) writer(in io.Reader) {
	scanner := bufio.NewScanner(in)
	sig := make(chan bool)
	for scanner.Scan() {
		if w.swapOut != nil {
			w.out = w.swapOut
			w.swapOut = nil
		}
		// If the writer is closed, then exit
		if w.out == nil {
			log.Println("Writer routine: No Listeners, exitting")
			return
		}

		// Do in a coroutine since we don't want to block
		go func() {
			// TODO: ONE coroutine will get blocked if a log write attempt was made juat before we detached
			// TODO: handle panic here since w.out might become nil
			// TODO: Check for error writing
			w.out.Write(scanner.Bytes())
			// Scan strips trailing end-of-line marker, add it back since
			// attatched listeners may need it for their own scan

			w.out.Write([]byte("\n"))
			//fmt.Println("\n--------LINE END --------")
			sig <- true
		}()

		// If write isn't done within 500ms, return
		select {
		case <-time.After(time.Millisecond * 500):
			log.Println("TIMOUT WAITING TO WRITE")
		case <-sig:
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "There was an error with the scanner", err)
	}
}

func (w *EventStream) Error(msg string, v ...interface{}) {
	if w.out != nil {
		w.emitEvent(Error, msg, v)
	}
}

func (w *EventStream) Warn(msg string, v ...interface{}) {
	if w.out != nil {
		w.emitEvent(Warning, msg, v)
	}
}

func (w *EventStream) Metric(msg string, v ...interface{}) {
	if w.out != nil {
		w.emitEvent(Metric, msg, v)
	}
}

func (w *EventStream) Info(msg string, v ...interface{}) {
	if w.out != nil {
		w.emitEvent(Info, msg, v)
	}
}

func (w *EventStream) emitEvent(t EventType, msg string, v ...interface{}) {
	e := Event{
		Type: t.String(),
		Name: w.name,
		Time: time.Now(), // TODO: Format time properly
		Node: w.node,
		Body: EventBody{
			Message: msg,
			Object:  v,
		},
	}

	var js []byte
	var err error
	if w.pretty {
		js, err = json.MarshalIndent(e, "", "\t")
	} else {
		js, err = json.Marshal(e)
	}
	if err != nil {
		log.Println("EventWriter: Error Marshaling, Ignoring event: ", err)
		return
	}

	// Use the logger we created to print. This takes care of sync when more
	// than one routine is using this EventWriter
	w.log.Println(string(js))
}
