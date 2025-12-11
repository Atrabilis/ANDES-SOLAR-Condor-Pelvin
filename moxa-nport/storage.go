package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// StorageManager coordinates writes to multiple storage targets.
type StorageManager struct {
	dests []influxDestination
}

type influxDestination struct {
	name        string
	baseURL     string
	org         string
	bucket      string
	measurement string
	token       string
	client      *http.Client
}

func NewStorageManager(cfg StorageConfig) *StorageManager {
	var dests []influxDestination

	makeDest := func(t StorageTarget) *influxDestination {
		if strings.ToLower(t.DBType) != "influxdb2" {
			return nil
		}
		if t.DBURL == "" || t.DBOrg == "" || t.DBBucket == "" {
			return nil
		}
		meas := t.DBMeasurement
		if meas == "" {
			meas = "registers"
		}
		return &influxDestination{
			name:        t.Name,
			baseURL:     strings.TrimRight(t.DBURL, "/"),
			org:         t.DBOrg,
			bucket:      t.DBBucket,
			measurement: meas,
			token:       t.DBToken,
			client: &http.Client{
				Timeout: 5 * time.Second,
			},
		}
	}

	for _, t := range cfg.Local {
		if d := makeDest(t); d != nil {
			dests = append(dests, *d)
		} else {
			fmt.Printf("skipping local storage %q (unsupported or missing fields)\n", t.Name)
		}
	}
	for _, t := range cfg.Remotes {
		if d := makeDest(t); d != nil {
			dests = append(dests, *d)
		} else {
			fmt.Printf("skipping remote storage %q (unsupported or missing fields)\n", t.Name)
		}
	}

	if len(dests) == 0 {
		return nil
	}
	return &StorageManager{dests: dests}
}

// Store writes register values to all configured destinations. It keeps writing even if some targets fail.
func (sm *StorageManager) Store(port string, slaveID uint8, slaveName string, values []RegisterValue, ts time.Time) {
	if sm == nil || len(values) == 0 {
		return
	}

	for _, dest := range sm.dests {
		body := buildLineProtocol(dest.measurement, port, slaveID, slaveName, values, ts)
		if body == "" {
			continue
		}

		writeURL := fmt.Sprintf("%s/api/v2/write?org=%s&bucket=%s&precision=ns",
			dest.baseURL,
			url.QueryEscape(dest.org),
			url.QueryEscape(dest.bucket),
		)
		req, err := http.NewRequest(http.MethodPost, writeURL, strings.NewReader(body))
		if err != nil {
			fmt.Printf("storage %q: build request failed: %v\n", dest.name, err)
			continue
		}
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")
		if dest.token != "" {
			req.Header.Set("Authorization", "Token "+dest.token)
		}

		resp, err := dest.client.Do(req)
		if err != nil {
			fmt.Printf("storage %q: write error: %v\n", dest.name, err)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 300 {
			msg := strings.TrimSpace(string(respBody))
			if msg == "" {
				msg = resp.Status
			}
			fmt.Printf("storage %q: write failed status=%s body=%s\n", dest.name, resp.Status, msg)
			continue
		}
	}
}

func buildLineProtocol(measurement string, port string, slaveID uint8, slaveName string, values []RegisterValue, ts time.Time) string {
	if measurement == "" {
		return ""
	}
	var b strings.Builder
	timestamp := ts.UnixNano()
	for _, v := range values {
		if v.Name == "" {
			continue
		}
		b.WriteString(escapeTag(measurement))
		b.WriteString(",port=")
		b.WriteString(escapeTag(port))
		b.WriteString(",slave=")
		b.WriteString(fmt.Sprintf("%d", slaveID))
		if slaveName != "" {
			b.WriteString(",slave_name=")
			b.WriteString(escapeTag(slaveName))
		}
		b.WriteString(",register=")
		b.WriteString(fmt.Sprintf("%d", v.Register))
		b.WriteString(",register_name=")
		b.WriteString(escapeTag(v.Name))
		b.WriteString(" ")
		b.WriteString(fieldKey(v))
		b.WriteString("=")
		b.WriteString(formatFieldValue(v))
		b.WriteString(" ")
		b.WriteString(fmt.Sprintf("%d", timestamp))
		b.WriteByte('\n')
	}
	return b.String()
}

func formatFieldValue(v RegisterValue) string {
	switch strings.ToLower(v.Type) {
	case "float", "float32", "float64":
		return strconv.FormatFloat(v.Value, 'f', -1, 64)
	default:
		return fmt.Sprintf("%di", int64(v.Value))
	}
}

func fieldKey(v RegisterValue) string {
	name := strings.TrimSpace(v.Name)
	if name == "" {
		name = fmt.Sprintf("register_%d", v.Register)
	}
	return escapeTag(strings.ToLower(name))
}

func escapeTag(v string) string {
	v = strings.ReplaceAll(v, "\\", "\\\\")
	v = strings.ReplaceAll(v, " ", "\\ ")
	v = strings.ReplaceAll(v, ",", "\\,")
	v = strings.ReplaceAll(v, "=", "\\=")
	return v
}
