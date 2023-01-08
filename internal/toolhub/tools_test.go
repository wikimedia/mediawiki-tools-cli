package toolhub

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"gitlab.wikimedia.org/repos/releng/cli/internal/util/mocks"
)

func TestClient_GetTool(t *testing.T) {
	type args struct {
		name    string
		options *ToolsOptions
	}

	mocks.GetDoFunc = func(*http.Request) (*http.Response, error) {
		return nil, errors.New(
			"Error from web server",
		)
	}

	tests := []struct {
		name    string
		doFunc  func(req *http.Request) (*http.Response, error)
		args    args
		want    *Tool
		wantErr bool
	}{
		{
			name: "Error for webserver",
			doFunc: func(*http.Request) (*http.Response, error) {
				return nil, errors.New(
					"Error from web server",
				)
			},
			args: args{
				name: "first",
			},
			wantErr: true,
		},
		{
			name: "Successful get",
			doFunc: func(*http.Request) (*http.Response, error) {
				// TODO add more things to the output if we care?
				json := `{"name":"toolforge-add","title":"Addshore's tools and services","description":"desc","url":"https://toolsadmin.wikimedia.org/tools/id/add","keywords":[]}`
				r := ioutil.NopCloser(bytes.NewReader([]byte(json)))
				return &http.Response{
					StatusCode: 200,
					Body:       r,
				}, nil
			},
			args: args{
				name: "toolforge-add",
			},
			wantErr: false,
			want: &Tool{
				Name:        "toolforge-add",
				Title:       "Addshore's tools and services",
				Description: "desc",
				URL:         "https://toolsadmin.wikimedia.org/tools/id/add",
				Keywords:    []string{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				BaseURL:    "someUrl...",
				HTTPClient: &mocks.MockClient{},
			}
			mocks.GetDoFunc = tt.doFunc
			got, err := c.GetTool(context.Background(), tt.args.name, tt.args.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.GetTool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.GetTool() = %v, want %v", got, tt.want)
			}
		})
	}
}
