package test

import (
    "os"
    "io"
    "time"
    "errors"
    "bufio"
    "bytes"
    "crypto/tls"
    "crypto/x509"
    "context"
    "net"
    "net/url"
    "net/http"
    "net/http/httptest"
    "net/http/httputil"
    "testing"
    p "caching-proxy/proxy"
)

var (
    port = ":16365"

    whiteContent = "http\nwhitelist\ngraylist"
    blackContent = "blacklist\ngraylist"
    whitelisted  = "whitelist"
    blacklisted  = "blacklist"
    graylisted   = "graylist"
    nonmatched   = "non-matching"
)

func create(path string, content... string) (err error) {
    file, err := os.Create(path)
    defer file.Close()
    file.WriteString(content[0])
    return
}

func createProxy(t *testing.T) (proxy *p.Proxy) {
    t.Helper()

    whitePath := os.TempDir() + "/test-proxy-white.txt"
    blackPath := os.TempDir() + "/test-proxy-black.txt"

    if err := create(whitePath, whiteContent); err != nil {
        t.Fatalf("Couldn't create whitelist file: %s", err)
        return
    }

    if err := create(blackPath, blackContent); err != nil {
        t.Fatalf("Coudln't create whitelist files: %s", err)
        return
    }

    proxy, err := p.New(
        port,
        whitePath,
        blackPath,
        "file:testdb?mode=memory",
        os.TempDir() + "/test-proxy-cert.cert",
        os.TempDir() + "/test-proxy-key.key",
    )
    if err != nil {
        t.Fatalf("Couldn't create a new proxy: %s", err)
        return
    }

    if transport, ok := proxy.Client.Transport.(*http.Transport); ok {
        if transport.TLSClientConfig == nil {
            transport.TLSClientConfig = &tls.Config{}
        }
        transport.TLSClientConfig.InsecureSkipVerify = true
    }
    return

}

// Create New Proxy
func TestCreateProxy(t *testing.T) {
    _ = createProxy(t)
}

// Test Request Matching
func TestMatchRequest(t *testing.T) {
    proxy := createProxy(t)

    values := [4]string {
        whitelisted,
        blacklisted,
        graylisted,
        nonmatched,
    }
    var body *bytes.Buffer
    var req *http.Request
    var expected bool

    for i, val1 := range values {
        for j, val2 := range values {
            body = bytes.NewBuffer([]byte{})
            req, _ = http.NewRequest("GET", val1, body)
            req.Header.Add("Referer", val2)
            expected = i == 0 || j == 0

            if proxy.Match(req) != expected {
                if expected {
                    t.Errorf("Didn't match whitelisted request\nURL: %s\nReferer: %s", req.URL.String(), req.Referer())
                } else {
                    t.Errorf("Matched blacklisted request\nURL: %s\nReferer: %s", req.URL.String(), req.Referer())
                }
            }
        }
    }

}

// Running Proxy
func TestRunningProxy(t *testing.T) {
    proxy := createProxy(t)
    
    ch := make(chan error, 1)
    go func() {
        ch <- proxy.Run()
    }()

    t.Cleanup(func() {
        if err := proxy.Close(); err != nil {
            t.Logf("Error closing proxy: %s", err)
        }
    })

    select {
    case runErr := <-ch:
        if runErr != nil && !errors.Is(runErr, http.ErrServerClosed) {
            t.Fatalf("Proxy failed to run: %s", runErr)
        }
    case <-time.After(100 * time.Millisecond):
    }
}

// Save&Load Response
func TestSaveLoadResponse(t *testing.T) {
	proxy := createProxy(t)

	reqBodyBytes := []byte("request")
	req, _ := http.NewRequest("GET", "http://example.com", bytes.NewBuffer(reqBodyBytes))
	req.Header.Add("Referer", "http://example.com")

	respBodyBytes := []byte("response")
	resp := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"X-Response-Header": []string{"true"},
		},
		Body: io.NopCloser(bytes.NewBuffer(respBodyBytes)),
	}

    err := proxy.SaveResponse(respBodyBytes, resp, req)
	if err != nil {
		t.Fatalf("Failed to save a response: %s", err)
	}

	var w bytes.Buffer
    ok := proxy.LoadResponse(&w, req, true)
	if !ok {
		t.Fatalf("Failed to load the saved response")
	}

	loadedResp, err := http.ReadResponse(bufio.NewReader(&w), req)
	if err != nil {
		t.Fatalf("Failed to parse the loaded response: %s", err)
	}
	defer loadedResp.Body.Close()

	savedDump, _ := httputil.DumpResponse(resp, true)
	resp.Body = io.NopCloser(bytes.NewBuffer(respBodyBytes)) 
	
	loadedDump, err := httputil.DumpResponse(loadedResp, true)
	if err != nil {
		t.Fatalf("Failed to dump loaded response: %s", err)
	}

	if !bytes.Equal(savedDump, loadedDump) {
		t.Fatalf("The loaded response isn't equal to the saved response\nSaved:\n%s\nLoaded:\n%s", savedDump, loadedDump)
	}
}

// Save/Update Response
func TestSaveUpdateResponse(t *testing.T) {
	proxy := createProxy(t)

	req, _ := http.NewRequest("GET", "http://example.com", bytes.NewBuffer([]byte("Request")))
	req.Header.Add("Referer", "http://example.com")

	respFirstBodyBytes := []byte("First response")
	respFirst := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"X-Response-Header": []string{"true"},
		},
		Body: io.NopCloser(bytes.NewBuffer(respFirstBodyBytes)),
	}

	respSecondBodyBytes := []byte("Second response")
	respSecond := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"X-Response-Header": []string{"true"},
		},
		Body: io.NopCloser(bytes.NewBuffer(respSecondBodyBytes)),
	}

    err := proxy.SaveResponse(respFirstBodyBytes, respFirst, req)
	if err != nil {
		t.Fatalf("Failed to save a response: %s", err)
	}

	err = proxy.SaveResponse(respSecondBodyBytes, respSecond, req)
	if err != nil {
		t.Fatalf("Failed to save a response: %s", err)
	}

	var w bytes.Buffer
    ok := proxy.LoadResponse(&w, req, true)
	if !ok {
		t.Fatalf("Failed to load the saved response")
	}

	loadedResp, err := http.ReadResponse(bufio.NewReader(&w), req)
	if err != nil {
		t.Fatalf("Failed to parse the loaded response: %s", err)
	}
	defer loadedResp.Body.Close()

	firstDump, _ := httputil.DumpResponse(respFirst, true)
	respFirst.Body = io.NopCloser(bytes.NewBuffer(respFirstBodyBytes)) 

	secondDump, _ := httputil.DumpResponse(respSecond, true)
	respSecond.Body = io.NopCloser(bytes.NewBuffer(respSecondBodyBytes)) 
	
	loadedDump, err := httputil.DumpResponse(loadedResp, true)
	if err != nil {
		t.Fatalf("Failed to dump loaded response: %s", err)
	}

    if bytes.Equal(loadedDump, firstDump) {
        t.Fatalf("Response wasn't updated, so first page was returned from cache:\nFirst: %s\nSecond: %s", firstDump, secondDump)
    } else if !bytes.Equal(loadedDump, secondDump) {
        t.Fatalf("New response was returned that doesn't match neither first, nor second:\nFirst:%s\nSecond:%s\nReturned:%s", firstDump, secondDump, loadedDump)
    }
}

// Save Unique Methods
func TestSaveUniqueMethods(t *testing.T) {
    proxy := createProxy(t)

	reqGET, _ := http.NewRequest("GET", "http://example.com", bytes.NewBuffer([]byte("GET Request")))
	reqGET.Header.Add("Referer", "http://example.com")

	respGETBodyBytes := []byte("GET Response")
	respGET := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"X-Response-Header": []string{"true"},
		},
		Body: io.NopCloser(bytes.NewBuffer(respGETBodyBytes)),
	}

	reqPOST, _ := http.NewRequest("POST", "http://example.com", bytes.NewBuffer([]byte("Request")))
	reqPOST.Header.Add("Referer", "http://example.com")

	respPOSTBodyBytes := []byte("POST Response")
	respPOST := &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"X-Response-Header": []string{"true"},
		},
		Body: io.NopCloser(bytes.NewBuffer(respPOSTBodyBytes)),
	}

    err := proxy.SaveResponse(respGETBodyBytes, respGET, reqGET)
	if err != nil {
		t.Fatalf("Failed to save a response: %s", err)
	}

	err = proxy.SaveResponse(respPOSTBodyBytes, respPOST, reqPOST)
	if err != nil {
		t.Fatalf("Failed to save a response: %s", err)
	}

	var w bytes.Buffer
    ok := proxy.LoadResponse(&w, reqGET, true)
	if !ok {
		t.Fatalf("Failed to load the saved response")
	}

	loadedRespGET, err := http.ReadResponse(bufio.NewReader(&w), reqGET)
	if err != nil {
		t.Fatalf("Failed to parse the loaded response: %s", err)
	}
	defer loadedRespGET.Body.Close()

    ok = proxy.LoadResponse(&w, reqPOST, true)
	if !ok {
		t.Fatalf("Failed to load the saved response")
	}

	loadedRespPOST, err := http.ReadResponse(bufio.NewReader(&w), reqPOST)
	if err != nil {
		t.Fatalf("Failed to parse the loaded response: %s", err)
	}
	defer loadedRespPOST.Body.Close()

	GETDump, _ := httputil.DumpResponse(respGET, true)
	respGET.Body = io.NopCloser(bytes.NewBuffer(respGETBodyBytes)) 

	POSTDump, _ := httputil.DumpResponse(respPOST, true)
	respPOST.Body = io.NopCloser(bytes.NewBuffer(respPOSTBodyBytes)) 
	
	loadedGETDump, err := httputil.DumpResponse(loadedRespGET, true)
	if err != nil {
		t.Fatalf("Failed to dump loaded response: %s", err)
	}

	loadedPOSTDump, err := httputil.DumpResponse(loadedRespPOST, true)
	if err != nil {
		t.Fatalf("Failed to dump loaded response: %s", err)
	}

    if bytes.Equal(loadedGETDump, GETDump) && bytes.Equal(loadedPOSTDump, POSTDump) {
        return
    }

    if !bytes.Equal(loadedGETDump, GETDump) {
        t.Errorf("The loaded GET response isn't equal to the saved GET response:\nSaved:%s\nLoaded:%s", GETDump, loadedGETDump)
    }

    if !bytes.Equal(loadedPOSTDump, POSTDump) {
        t.Errorf("The loaded POST response isn't equal to the saved POST response:\nSaved:%s\nLoaded:%s", POSTDump, loadedPOSTDump)
    }
}

// Handle HTTP
func TestHandleHTTP(t *testing.T) {
	proxy := createProxy(t)

	ch := make(chan error, 1)
	go func() {
		ch <- proxy.Run()
	}()

	t.Cleanup(func() {
		_ = proxy.Close()
	})

	time.Sleep(100 * time.Millisecond)

	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer targetServer.Close()

	proxyURL, _ := url.Parse("http://127.0.0.1" + port)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(targetServer.URL)
	if err != nil {
		t.Fatalf("Failed to send HTTP online request through proxy: %s", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if bytes.Contains(bodyBytes, []byte("\r\n\r\n")) {
		parts := bytes.SplitN(bodyBytes, []byte("\r\n\r\n"), 2)
		bodyBytes = parts[1]
	}

	if string(bodyBytes) != "hello world" {
		t.Errorf("Received wrong site content in HTTP online request:\nExpected: \"hello world\"\nReceived: \"%s\"", string(bodyBytes))
	}

	targetServer.Close()

	resp, err = client.Get(targetServer.URL)
	if err != nil {
		t.Fatalf("Failed to send HTTP offline request through proxy: %s", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ = io.ReadAll(resp.Body)

	if bytes.Contains(bodyBytes, []byte("\r\n\r\n")) {
		parts := bytes.SplitN(bodyBytes, []byte("\r\n\r\n"), 2)
		bodyBytes = parts[1]
	}

	if string(bodyBytes) != "hello world" {
		t.Errorf("Received wrong site content in HTTP offline request:\nExpected: \"hello world\"\nReceived: \"%s\"", string(bodyBytes))
	}
}

// Handle HTTPS
func TestHandleHTTPS(t *testing.T) {
	proxy := createProxy(t)

	ch := make(chan error, 1)
	go func() {
		ch <- proxy.Run()
	}()

	t.Cleanup(func() {
		_ = proxy.Close()
	})

	time.Sleep(100 * time.Millisecond)

	targetServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer targetServer.Close()

	certPool := x509.NewCertPool()
	proxyCertBytes, err := os.ReadFile(os.TempDir() + "/test-proxy-cert.cert")
	if err != nil {
		t.Fatalf("Failed to read proxy cert for client configuration: %s", err)
	}
	certPool.AppendCertsFromPEM(proxyCertBytes)

	proxyURL, _ := url.Parse("http://127.0.0.1" + port)
	
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				RootCAs:            certPool,
				InsecureSkipVerify: true,
			},
			DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := &net.Dialer{}
				conn, err := dialer.DialContext(ctx, network, "127.0.0.1"+port)
				if err != nil {
					return nil, err
				}
				return tls.Client(conn, &tls.Config{
					RootCAs:            certPool,
					InsecureSkipVerify: true,
				}), nil
			},
		},
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(targetServer.URL)
	if err != nil {
		t.Fatalf("Failed to send HTTPS online request through proxy: %s", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if bytes.Contains(body, []byte("\r\n\r\n")) {
		parts := bytes.SplitN(body, []byte("\r\n\r\n"), 2)
		body = parts[1]
	}
	
	if string(body) != "hello world" {
		t.Errorf("Received wrong site content in HTTPS online request:\nExpected: \"hello world\"\nReceived: \"%s\"", string(body))
	}

	targetServer.Close()

	resp, err = client.Get(targetServer.URL)
	if err != nil {
		t.Fatalf("Failed to send HTTPS offline request through proxy: %s", err)
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)
	if bytes.Contains(body, []byte("\r\n\r\n")) {
		parts := bytes.SplitN(body, []byte("\r\n\r\n"), 2)
		body = parts[1]
	}
	
	if string(body) != "hello world" {
		t.Errorf("Received wrong site content in HTTPS offline request:\nExpected: \"hello world\"\nReceived: \"%s\"", string(body))
	}
}
