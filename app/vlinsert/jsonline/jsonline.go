package jsonline

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaLogs/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaLogs/lib/logstorage"
)

// RequestHandler processes jsonline insert requests
func RequestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
		return
	case http.MethodPost:
		w.Header().Add("Content-Type", "application/json")
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	startTime := time.Now()
	requestsTotal.Inc()

	cp, err := insertutil.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if err := insertutil.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	reader, err := protoparserutil.GetUncompressedReader(r.Body, encoding)
	if err != nil {
		logger.Errorf("cannot decode jsonline request: %s", err)
		return
	}
	defer protoparserutil.PutUncompressedReader(reader)

	lmp := cp.NewLogMessageProcessor("jsonline", true)
	streamName := fmt.Sprintf("remoteAddr=%s, requestURI=%q", httpserver.GetQuotedRemoteAddr(r), r.RequestURI)
	err = processStreamInternal(streamName, reader, cp.TimeFields, cp.MsgFields, lmp)
	lmp.MustClose()
	if err != nil {
		httpserver.Errorf(w, r, "cannot process jsonline request; error: %s", err)
		return
	}

	requestDuration.UpdateDuration(startTime)
}

func processStreamInternal(streamName string, r io.Reader, timeFields, msgFields []string, lmp insertutil.LogMessageProcessor) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	lr := insertutil.NewLineReader(streamName, wcr)

	n := 0
	errors := 0
	var lastError error
	for {
		ok, err := readLine(lr, timeFields, msgFields, lmp)
		wcr.DecConcurrency()
		if err != nil {
			lastError = err
			errors++
			logger.Warnf("jsonline: cannot read line #%d in /jsonline request: %s", n, err)
		}
		if !ok {
			break
		}
		n++
	}
	errorsTotal.Add(errors)

	if errors > 0 && n == errors {
		// Return an error if no logs were processed and there were errors
		return lastError
	}

	return nil
}

func readLine(lr *insertutil.LineReader, timeFields, msgFields []string, lmp insertutil.LogMessageProcessor) (bool, error) {
	var line []byte
	for len(line) == 0 {
		if !lr.NextLine() {
			err := lr.Err()
			return false, err
		}
		line = lr.Line
	}

	p := logstorage.GetJSONParser()
	defer logstorage.PutJSONParser(p)

	p.Init(line)
	for p.NextMessage() {
		if err := p.Error(); err != nil {
			return true, err
		}
		ts, err := insertutil.ExtractTimestampFromFields(timeFields, p.Fields)
		if err != nil {
			return true, err
		}
		logstorage.RenameField(p.Fields, msgFields, "_msg")
		lmp.AddRow(ts, p.Fields, nil)
	}
	if err := p.Error(); err != nil {
		return true, err
	}

	return true, nil
}

var (
	requestsTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/jsonline"}`)
	errorsTotal   = metrics.NewCounter(`vl_http_errors_total{path="/insert/jsonline"}`)

	requestDuration = metrics.NewSummary(`vl_http_request_duration_seconds{path="/insert/jsonline"}`)
)
