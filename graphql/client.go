package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Client is the interface that the generated code calls into to actually make
// requests.
type Client interface {
	// MakeRequest must make a request to the client's GraphQL API.
	//
	// ctx is the context that should be used to make this request.  If context
	// is disabled in the genqlient settings, this will be set to
	// context.Background().
	//
	// req contains the data to be sent to the GraphQL server.  Typically GraphQL
	// APIs will expect it to simply be marshalled as JSON, but MakeRequest may
	// customize this.
	//
	// resp is the Response object into which the server's response will be
	// unmarshalled. Typically GraphQL APIs will return JSON which can be
	// unmarshalled directly into resp, but MakeRequest can customize it.
	// If the response contains an error, this must also be returned by
	// MakeRequest.  The field resp.Data will be prepopulated with a pointer
	// to an empty struct of the correct generated type (e.g. MyQueryResponse).
	MakeRequest(
		ctx context.Context,
		req *Request,
		resp *Response,
	) error
}

type client struct {
	httpClient Doer
	endpoint   string
	method     string
}

// NewClient returns a [Client] which makes requests to the given endpoint,
// suitable for most users.
//
// The client makes POST requests to the given GraphQL endpoint using standard
// GraphQL HTTP-over-JSON transport.  It will use the given [http.Client], or
// [http.DefaultClient] if a nil client is passed.
//
// The typical method of adding authentication headers is to wrap the client's
// [http.Transport] to add those headers.  See [example/main.go] for an
// example.
//
// [example/main.go]: https://github.com/Khan/genqlient/blob/main/example/main.go#L12-L20
func NewClient(endpoint string, httpClient Doer) Client {
	return newClient(endpoint, httpClient, http.MethodPost)
}

// NewClientUsingGet returns a [Client] which makes GET requests to the given
// endpoint suitable for most users who wish to make GET requests instead of
// POST.
//
// The client makes GET requests to the given GraphQL endpoint using a GET
// query, with the query, operation name and variables encoded as URL
// parameters.  It will use the given [http.Client], or [http.DefaultClient] if
// a nil client is passed.
//
// The client does not support mutations, and will return an error if passed a
// request that attempts one.
//
// The typical method of adding authentication headers is to wrap the client's
// [http.Transport] to add those headers.  See [example/main.go] for an
// example.
//
// [example/main.go]: https://github.com/Khan/genqlient/blob/main/example/main.go#L12-L20
func NewClientUsingGet(endpoint string, httpClient Doer) Client {
	return newClient(endpoint, httpClient, http.MethodGet)
}

func newClient(endpoint string, httpClient Doer, method string) Client {
	if httpClient == nil || httpClient == (*http.Client)(nil) {
		httpClient = http.DefaultClient
	}
	return &client{httpClient, endpoint, method}
}

// Doer encapsulates the methods from [*http.Client] needed by [Client].
// The methods should have behavior to match that of [*http.Client]
// (or mocks for the same).
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Request contains all the values required to build queries executed by
// the [Client].
//
// Typically, GraphQL APIs will accept a JSON payload of the form
//
//	{"query": "query myQuery { ... }", "variables": {...}}`
//
// and Request marshals to this format.  However, MakeRequest may
// marshal the data in some other way desired by the backend.
type Request struct {
	// The literal string representing the GraphQL query, e.g.
	// `query myQuery { myField }`.
	Query string `json:"query"`
	// A JSON-marshalable value containing the variables to be sent
	// along with the query, or nil if there are none.
	Variables interface{} `json:"variables,omitempty"`
	// The GraphQL operation name. The server typically doesn't
	// require this unless there are multiple queries in the
	// document, but genqlient sets it unconditionally anyway.
	OpName string `json:"operationName"`
	// If this is true, request will do multipart upload file.
	UploadFile bool
}

// Response that contains data returned by the GraphQL API.
//
// Typically, GraphQL APIs will return a JSON payload of the form
//
//	{"data": {...}, "errors": {...}}
//
// It may additionally contain a key named "extensions", that
// might hold GraphQL protocol extensions. Extensions and Errors
// are optional, depending on the values returned by the server.
type Response struct {
	Data       interface{}            `json:"data"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
	Errors     gqlerror.List          `json:"errors,omitempty"`
}

func (c *client) MakeRequest(ctx context.Context, req *Request, resp *Response) error {
	var httpReq *http.Request
	var err error
	if c.method == http.MethodGet {
		httpReq, err = c.createGetRequest(req)
	} else {
		if req.UploadFile {
			httpReq, err = c.createUploadFileRequest(req)
		} else {
			httpReq, err = c.createPostRequest(req)
		}
	}

	if err != nil {
		return err
	}

	if !req.UploadFile || c.method == http.MethodGet {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	if ctx != nil {
		httpReq = httpReq.WithContext(ctx)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		var respBody []byte
		respBody, err = io.ReadAll(httpResp.Body)
		if err != nil {
			respBody = []byte(fmt.Sprintf("<unreadable: %v>", err))
		}
		return fmt.Errorf("returned error %v: %s", httpResp.Status, respBody)
	}

	err = json.NewDecoder(httpResp.Body).Decode(resp)
	if err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		return resp.Errors
	}
	return nil
}

type fileVariable struct {
	mapKey string
	file   Upload
}

// recursively find all the fields that are type Upload.
func findFiles(parentKey string, v reflect.Value) []*fileVariable {
	fileVariables := []*fileVariable{}

	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Type() == reflect.TypeOf(Upload{}) {
		file := v.Interface().(Upload)
		fileVariables = append(fileVariables, &fileVariable{
			mapKey: parentKey,
			file:   file,
		})
		return fileVariables
	}

	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			fieldName := v.Type().Field(i).Name
			jsonTag := v.Type().Field(i).Tag.Get("json")
			if jsonTag != "" && jsonTag != "-" {
				fieldName = jsonTag
			}
			fileVariables = append(fileVariables, findFiles(parentKey+"."+fieldName, field)...)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			fileVariables = append(fileVariables, findFiles(parentKey+"."+strconv.Itoa(i), elem)...)
		}
	}

	return fileVariables
}

func (c *client) createUploadFileRequest(req *Request) (*http.Request, error) {
	httpRequest, err := http.NewRequest(http.MethodPost, c.endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)
	defer bodyWriter.Close()

	// operations
	requestBody, _ := json.Marshal(req)
	err = bodyWriter.WriteField("operations", string(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error writing operations to body: %w", err)
	}

	// map
	mapData := ""
	fileVariables := findFiles("variables", reflect.ValueOf(req.Variables))
	// group files to avoid uploading duplicated files
	filesGroup := [][]*fileVariable{}
	for _, file := range fileVariables {
		foundDuplicated := false
		for group, fileGroup := range filesGroup {
			file2 := fileGroup[0]
			if file.file.FileName == file2.file.FileName {
				f1, err := io.ReadAll(file.file.Body)
				if err != nil {
					return nil, fmt.Errorf("error reading file: %w", err)
				}
				f2, err := io.ReadAll(file2.file.Body)
				if err != nil {
					return nil, fmt.Errorf("error reading file: %w", err)
				}
				file.file.Body = bytes.NewReader(f1)
				file2.file.Body = bytes.NewReader(f2)
				if bytes.Equal(f1, f2) {
					foundDuplicated = true
					filesGroup[group] = append(filesGroup[group], file)
					break
				}
			}
		}
		if !foundDuplicated {
			filesGroup = append(filesGroup, []*fileVariable{file})
		}
	}
	if len(filesGroup) > 0 {
		variablesString := []string{}
		for i, files := range filesGroup {
			variablesString = append(variablesString, fmt.Sprintf("\"%d\":[%s]", i, joinFilesMapKey(files)))
		}
		mapData = `{` + strings.Join(variablesString, ",") + `}`
	}
	err = bodyWriter.WriteField("map", mapData)
	if err != nil {
		return nil, fmt.Errorf("error writing map data to body: %w", err)
	}

	// files
	for i, file := range filesGroup {
		h := make(textproto.MIMEHeader)
		dispParams := map[string]string{"name": strconv.Itoa(i)}
		fileName := strings.TrimSpace(file[0].file.FileName)
		if fileName != "" {
			dispParams["filename"] = fileName
		}
		h.Set("Content-Disposition", mime.FormatMediaType("form-data", dispParams))
		b, err := io.ReadAll(file[0].file.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading file: %w", err)
		}
		h.Set("Content-Type", http.DetectContentType(b))
		ff, err := bodyWriter.CreatePart(h)
		if err != nil {
			return nil, fmt.Errorf("error create multipart header: %w", err)
		}
		_, err = ff.Write(b)
		if err != nil {
			return nil, fmt.Errorf("error writing file to body: %w", err)
		}
	}
	httpRequest.Body = io.NopCloser(bodyBuf)
	httpRequest.Header.Set("Content-Type", bodyWriter.FormDataContentType())

	return httpRequest, nil
}

func joinFilesMapKey(files []*fileVariable) string {
	fileKeys := make([]string, len(files))
	for i, v := range files {
		fileKeys[i] = fmt.Sprintf("\"%s\"", v.mapKey)
	}
	return strings.Join(fileKeys, ",")
}

func (c *client) createPostRequest(req *Request) (*http.Request, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest(
		c.method,
		c.endpoint,
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return httpReq, nil
}

func (c *client) createGetRequest(req *Request) (*http.Request, error) {
	parsedURL, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, err
	}

	queryParams := parsedURL.Query()
	queryUpdated := false

	if req.Query != "" {
		if strings.HasPrefix(strings.TrimSpace(req.Query), "mutation") {
			return nil, errors.New("client does not support mutations")
		}
		queryParams.Set("query", req.Query)
		queryUpdated = true
	}

	if req.OpName != "" {
		queryParams.Set("operationName", req.OpName)
		queryUpdated = true
	}

	if req.Variables != nil {
		variables, variablesErr := json.Marshal(req.Variables)
		if variablesErr != nil {
			return nil, variablesErr
		}
		queryParams.Set("variables", string(variables))
		queryUpdated = true
	}

	if queryUpdated {
		parsedURL.RawQuery = queryParams.Encode()
	}

	httpReq, err := http.NewRequest(
		c.method,
		parsedURL.String(),
		http.NoBody)
	if err != nil {
		return nil, err
	}

	return httpReq, nil
}
