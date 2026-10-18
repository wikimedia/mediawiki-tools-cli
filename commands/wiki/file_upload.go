package wiki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mwclient "cgt.name/pkg/go-mwclient"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
)

type fileUploadOptions struct {
	filename, comment, text, textFile string
	replace, ignoreWarnings, dryRun   bool
}

func NewWikiFileUploadCmd() *cobra.Command {
	var options fileUploadOptions
	cmd := &cobra.Command{
		Use:   "upload FILE",
		Short: "Upload a local file or a new version of an existing file",
		Long:  "Upload a local file with bot-password authentication. Use --text-file for the initial file description and license wikitext. New versions preserve the existing description. Replacing a file requires --replace; other upload warnings require --ignore-warnings.\n\nGuide: mw help guides wiki files",
		Args:  cobra.ExactArgs(1),
		Example: cobrautil.NormalizeExample(`
# Upload a video with a description and license
upload demo.webm --wiki commons.wikimedia.org --user 'Name@bot' --password-file password.txt --text-file description.wiki
# Upload a new version under the same wiki filename
upload demo.webm --wiki commons.wikimedia.org --user 'Name@bot' --password-file password.txt --filename 'Demo.webm' --replace --comment 'Refresh walkthrough'
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() || info.Size() == 0 {
				return fmt.Errorf("upload source must be a nonempty regular file")
			}
			if options.filename == "" {
				options.filename = filepath.Base(args[0])
			}
			if strings.HasPrefix(strings.ToLower(options.filename), "file:") {
				return fmt.Errorf("--filename must not include the File: namespace prefix")
			}
			if options.textFile != "" {
				text, err := os.ReadFile(options.textFile)
				if err != nil {
					return fmt.Errorf("read description: %w", err)
				}
				options.text = string(text)
			}
			endpoint := normalizeWiki(wiki)
			if options.dryRun {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Would upload %s as %s to %s (replace: %t)\n", args[0], options.filename, endpoint, options.replace)
				return err
			}
			if wikiAnon {
				return fmt.Errorf("file uploads require an authenticated account; omit --anon")
			}
			if wikiUser == "" || (wikiPassword == "" && wikiPasswordFile == "") {
				return fmt.Errorf("file uploads require --user and either --password or --password-file")
			}
			w, err := mwclient.New(endpoint, cli.HTTPUserAgent)
			if err != nil {
				return err
			}
			// SetHTTPClient preserves the library's cookie jar. Login, tokens and
			// multipart uploads then share the same authenticated session.
			httpClient := &http.Client{Timeout: 10 * time.Minute}
			w.SetHTTPClient(httpClient)
			if err := loginIfCredentialsProvided(w); err != nil {
				return err
			}
			w.Assert = mwclient.AssertUser
			token, err := w.GetToken(mwclient.CSRFToken)
			if err != nil {
				return err
			}
			result, err := uploadWikiFile(cmd.Context(), httpClient, endpoint, w.UserAgent, token, file, info.Size(), options)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Uploaded %s\n%s\n", result.Filename, result.ImageInfo.DescriptionURL)
			return err
		},
	}
	cmd.Flags().StringVar(&options.filename, "filename", "", "Wiki filename without File: (defaults to the local basename)")
	cmd.Flags().StringVar(&options.comment, "comment", cli.DisplayName+" upload", "Upload summary")
	cmd.Flags().StringVar(&options.text, "text", "", "Initial description and license wikitext for a new file")
	cmd.Flags().StringVar(&options.textFile, "text-file", "", "Read the initial description and license wikitext from a file")
	cmd.Flags().BoolVar(&options.replace, "replace", false, "Allow uploading a new version of an existing file")
	cmd.Flags().BoolVar(&options.ignoreWarnings, "ignore-warnings", false, "Accept other upload warnings (overwriting still requires --replace)")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "Show the upload target without contacting the wiki")
	cmd.MarkFlagsMutuallyExclusive("text", "text-file")
	_ = cmd.MarkFlagFilename("text-file")
	return cmd
}

type wikiUploadResult struct {
	Result    string                     `json:"result"`
	Filename  string                     `json:"filename"`
	FileKey   string                     `json:"filekey"`
	Warnings  map[string]json.RawMessage `json:"warnings"`
	ImageInfo struct {
		DescriptionURL string `json:"descriptionurl"`
	} `json:"imageinfo"`
}

type wikiUploadResponse struct {
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
	Upload wikiUploadResult `json:"upload"`
}

func uploadWikiFile(ctx context.Context, client *http.Client, endpoint, userAgent, token string, file *os.File, size int64, options fileUploadOptions) (wikiUploadResult, error) {
	fields := url.Values{
		"action": {"upload"}, "format": {"json"}, "formatversion": {"2"},
		"assert": {"user"}, "maxlag": {"5"}, "filename": {options.filename}, "comment": {options.comment},
	}
	if options.text != "" {
		fields.Set("text", options.text)
	}

	// Buffer only the multipart framing, streaming the file between the header
	// and footer. Put the CSRF token last, as recommended by the Action API.
	var framing bytes.Buffer
	writer := multipart.NewWriter(&framing)
	for key, values := range fields {
		if err := writer.WriteField(key, values[0]); err != nil {
			return wikiUploadResult{}, err
		}
	}
	if _, err := writer.CreateFormFile("file", filepath.Base(file.Name())); err != nil {
		return wikiUploadResult{}, err
	}
	header := append([]byte(nil), framing.Bytes()...)
	framing.Reset()
	if err := writer.WriteField("token", token); err != nil {
		return wikiUploadResult{}, err
	}
	if err := writer.Close(); err != nil {
		return wikiUploadResult{}, err
	}
	body := io.MultiReader(bytes.NewReader(header), file, bytes.NewReader(framing.Bytes()))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return wikiUploadResult{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = int64(len(header)+framing.Len()) + size
	result, err := sendWikiUpload(client, req, userAgent)
	if err != nil {
		return result, err
	}
	if result.Result == "Success" {
		return result, nil
	}
	if result.Result != "Warning" || len(result.Warnings) == 0 {
		return result, fmt.Errorf("unexpected upload result %q", result.Result)
	}
	var blocked []string
	for name, detail := range result.Warnings {
		exists := name == "exists" || name == "exists-normalized"
		if (exists && !options.replace) || (!exists && !options.ignoreWarnings) {
			flag := "--ignore-warnings"
			if exists {
				flag = "--replace"
			}
			blocked = append(blocked, fmt.Sprintf("%s: %s (review and use %s to accept)", name, detail, flag))
		}
	}
	if len(blocked) > 0 {
		sort.Strings(blocked)
		return result, fmt.Errorf("upload not published: %s", strings.Join(blocked, "; "))
	}
	if result.FileKey == "" {
		return result, fmt.Errorf("upload warning response omitted the filekey; upload not published")
	}
	fields.Set("filekey", result.FileKey)
	fields.Set("ignorewarnings", "1")
	fields.Set("token", token)
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(fields.Encode()))
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	result, err = sendWikiUpload(client, req, userAgent)
	if err != nil {
		return result, err
	}
	if result.Result != "Success" {
		return result, fmt.Errorf("upload not published: unexpected result %q", result.Result)
	}
	return result, nil
}

func sendWikiUpload(client *http.Client, req *http.Request, userAgent string) (wikiUploadResult, error) {
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return wikiUploadResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return wikiUploadResult{}, fmt.Errorf("upload returned HTTP %s", resp.Status)
	}
	var result wikiUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return result.Upload, fmt.Errorf("decode upload response: %w", err)
	}
	if result.Error != nil {
		return result.Upload, fmt.Errorf("upload failed (%s): %s", result.Error.Code, result.Error.Info)
	}
	return result.Upload, nil
}
