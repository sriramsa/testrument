// Package testrument provides tests
package testrument

import (
	"bufio"
	"errors"
	"log"
	"regexp"
	"testing"
	"time"
)

// We test things incrementally such that it is easy to debug. This creates redundant
// tests being performed, but analyzing failures are quick.
// Test cases
// - EventWriter
//		- Can create one.
//		- Can create more than one.
//		- Can write to the event stream with nobody attached
//			- Different Types
//				- Info
//				- Metric
//				- Warning
//				- Error
//			- Can Write Objects
//		- Can Write to the event stream with a attachment
// - Attaching to an event channel
//		- Can Attach to an existing stream and receive event
//		- More than one can attach
//			- Each receives events
//		- Can Detach from a stream
//		- WaitFor an event
//			- Different Types
//				- Info
//				- Metric
//				- Warning
//				- Error
//			- Timeout when event not found

// Verify that we can create a basic stream
func TestCreateEventStream(t *testing.T) {
	if _, err := createEventStream("unittest"); err != nil {
		t.Error("Creating NewEventStream failed : ", err)
		t.FailNow()
	}
}

// Verify that we can create multiple streams in the same package
func TestCreateMultipleEventStreams(t *testing.T) {
	if _, err := createEventStream("unittest"); err != nil {
		t.Error("Creating NewEventStream failed : ", err)
		t.FailNow()
	}
	if _, err := createEventStream("unittest1"); err != nil {
		t.Error("Creating Second NewEventStream failed : ", err)
		t.FailNow()
	}
}

// Verify that one can write to a stream even when nobody is attached
func TestWriteToStreamNoReader(t *testing.T) {
	es, err := createEventStream("unittest")
	if err != nil {
		t.Error("Creating NewEventStream failed : ", err)
		t.FailNow()
	}
	es.Info("This is Info")
	es.Warn("This is Warn")
	es.Metric("This is Metric")
	es.Error("This is Error")
}

// Verify that one can write an object to a stream even when nobody is attached.
// Passing object JSON marshalls the same.
func TestWriteObjectToStreamNoReader(t *testing.T) {
	es, err := createEventStream("unittest")
	if err != nil {
		t.Error("Creating NewEventStream failed : ", err)
		t.FailNow()
	}
	logObj := log.New(nil, "test", 0)
	es.Info("This is log object :", logObj)
	es.Warn("This is Warn :", logObj)
	es.Metric("This is Metric :", logObj)
	es.Error("This is Error :", logObj)
}

// Verify that one can write to a stream even when nobody is attached
func TestWriteToStreamWithAttachment(t *testing.T) {
	emit, err := createEventStream("unittest")
	if err != nil {
		t.Error("Creating NewEventStream failed : ", err)
		t.FailNow()
	}

	wait := make(chan struct{}, 0)
	at := NewAttach(emit)

	// Spin up a go routine to look for events
	go func() {
		scanner := bufio.NewScanner(at.Reader)
		var i int
		for scanner.Scan() {
			s := scanner.Text()
			t.Log("Event Received :", s)
			ri, _ := regexp.Compile("InfoEvent")
			rw, _ := regexp.Compile("WarningEvent")
			rm, _ := regexp.Compile("MetricEvent")
			re, _ := regexp.Compile("ErrorEvent")
			if ri.FindString(s) != "" ||
				rw.FindString(s) != "" ||
				rm.FindString(s) != "" ||
				re.FindString(s) != "" {
				i = i + 1
			}
			if i == 4 {
				t.Log("All Events Received. Exitting")
				close(wait)
				return
			}
		}
	}()

	// Generate 4 new events
	emit.Info("InfoEvent")
	emit.Warn("WarningEvent")
	emit.Metric("MetricEvent")
	emit.Error("ErrorEvent")

	// Wait for them for 3 seconds
	if err := channelWait(wait, time.Second*3); err != nil {
		t.Error("Events not received within 3 seconds. Failing.")
		t.FailNow()
	}
}

// Verify that one can write an object to a stream even when nobody is attached.
// Passing object JSON marshalls the same.
func TestWriteObjectToStreamWithAttachment(t *testing.T) {
	es, err := createEventStream("unittest")
	if err != nil {
		t.Error("Creating NewEventStream failed : ", err)
		t.FailNow()
	}
	logObj := log.New(nil, "test", 0)
	es.Info("This is log object :", logObj)
	es.Warn("This is Warn :", logObj)
	es.Metric("This is Metric :", logObj)
	es.Error("This is Error :", logObj)
}

// Test Helpers
func createEventStream(pkg string) (es *EventStream, err error) {
	es = NewEventStream(pkg, false)
	if es == nil {
		err = errors.New("NewEventStream returned nil")
	}
	return es, err
}

func channelWait(ch chan struct{}, tm time.Duration) error {
	select {
	case <-time.After(tm):
		return errors.New("Timeout")
	case <-ch:
	}

	return nil
}
