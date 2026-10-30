package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
	"github.com/spf13/cobra"
	"gitlab.wikimedia.org/repos/releng/cli/internal/cli"
	cobrautil "gitlab.wikimedia.org/repos/releng/cli/internal/util/cobra"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	maxConvertedImageBytes  = 128 << 20
	maxConvertedImagePixels = 40_000_000
)

type wikiDownloadInfo struct {
	URL        string `json:"url"`
	ThumbURL   string `json:"thumburl"`
	ThumbError string `json:"thumberror"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

func NewWikiFileDownloadCmd() *cobra.Command {
	var format, output string
	var width int
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "download FILE",
		Short: "Download an original wiki file or convert its rendered preview to PNG or JPEG",
		Long:  "Download a wiki filename (with or without File:) or a file-description URL. File-description URLs infer --wiki when it is omitted. The default format, original, preserves the uploaded bytes.\n\n--format png or jpeg downloads MediaWiki's rendered preview and encodes the requested format locally. SVG rendering happens on the wiki; local conversion accepts PNG, JPEG, GIF and WebP previews. Conversion exports a single still image. PNG preserves transparency; JPEG uses a white background.\n\n--width sets the exact converted width in pixels and preserves the aspect ratio, even when the wiki returns a larger thumbnail. The default is the source width, or 1024 pixels when unknown. Large images may require a smaller width.\n\nThe default output filename is based on the wiki filename with the converted extension. Existing files require --overwrite. Use --output - to write file bytes to standard output.\n\nGuide: mw help guides wiki files",
		Args:  cobra.ExactArgs(1),
		Example: cobrautil.NormalizeExample(`
# Save a genuine PNG from an SVG file on Commons
download 'https://commons.wikimedia.org/wiki/File:Logo_of_a-wikimedia-cli.svg' --format png --width 1024 --output logo.png
# Download the original uploaded file
download 'Logo_of_a-wikimedia-cli.svg' --wiki commons.wikimedia.org
`),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			_, origin, err := wikiDownloadSource(args[0])
			if err != nil {
				return err
			}
			if origin != "" && !cmd.Flags().Changed("wiki") {
				return cmd.Flags().Set("wiki", origin)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			format = strings.ToLower(format)
			if format == "jpg" {
				format = "jpeg"
			}
			if format != "original" && format != "png" && format != "jpeg" {
				return fmt.Errorf("--format must be original, png, or jpeg")
			}
			if width < 0 || width > 16384 {
				return fmt.Errorf("--width must be between 0 and 16384")
			}
			if format == "original" && width != 0 {
				return fmt.Errorf("use --format png or jpeg with --width")
			}
			title, origin, err := wikiDownloadSource(args[0])
			if err != nil {
				return err
			}
			if origin != "" {
				source, _ := url.Parse(origin)
				endpoint, err := url.Parse(normalizeWiki(wiki))
				if err != nil {
					return err
				}
				if !strings.EqualFold(source.Host, endpoint.Host) {
					return fmt.Errorf("file-description URL and --wiki must refer to the same host")
				}
			}
			if output == "" {
				_, basename, _ := strings.Cut(title, ":")
				output = path.Base(strings.ReplaceAll(basename, "\\", "/"))
				if format != "original" {
					output = strings.TrimSuffix(output, path.Ext(output)) + "." + format
				}
			}
			if output == "" || output == "." || output == ".." {
				return fmt.Errorf("choose an output filename with --output")
			}
			if output != "-" && !overwrite {
				if _, err := os.Lstat(output); err == nil {
					return fmt.Errorf("output %q already exists; use --overwrite to replace it", output)
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			httpClient := cli.NewHTTPClient()
			httpClient.Timeout = 5 * time.Minute
			w, err := mwclient.New(normalizeWiki(wiki), cli.HTTPUserAgent)
			if err != nil {
				return err
			}
			w.SetHTTPClient(httpClient)
			if err := loginIfCredentialsProvided(w, httpClient); err != nil {
				return err
			}
			if !wikiAnon && wikiUser != "" {
				w.Assert = mwclient.AssertUser
			}
			info, err := queryWikiDownload(w, title, width)
			if err != nil {
				return err
			}
			assetURL := info.URL
			if format != "original" {
				if width == 0 {
					width = info.Width
					if width <= 0 {
						width = 1024
					}
					if width > 16384 {
						return fmt.Errorf("source is too large to convert; choose a smaller --width")
					}
					info, err = queryWikiDownload(w, title, width)
					if err != nil {
						return err
					}
				}
				if info.ThumbError != "" {
					return fmt.Errorf("wiki thumbnail rendering failed: %s", info.ThumbError)
				}
				if info.ThumbURL == "" {
					return fmt.Errorf("wiki did not provide a rendered preview for %s", title)
				}
				assetURL = info.ThumbURL
			}
			// Download and validate before touching the requested output path.
			asset, err := os.CreateTemp("", "mwcli-download-*")
			if err != nil {
				return err
			}
			defer os.Remove(asset.Name())
			defer asset.Close()
			if err := fetchWikiDownload(cmd.Context(), httpClient, assetURL, format, asset); err != nil {
				return err
			}
			if _, err := asset.Seek(0, io.SeekStart); err != nil {
				return err
			}
			var converted image.Image
			if format != "original" {
				converted, err = decodeWikiDownload(asset, width)
				if err != nil {
					return err
				}
			}
			write := func(destination io.Writer) error {
				switch format {
				case "png":
					return png.Encode(destination, converted)
				case "jpeg":
					background := image.NewRGBA(converted.Bounds())
					draw.Draw(background, background.Bounds(), image.White, image.Point{}, draw.Src)
					draw.Draw(background, background.Bounds(), converted, converted.Bounds().Min, draw.Over)
					return jpeg.Encode(destination, background, &jpeg.Options{Quality: 90})
				default:
					_, err := io.Copy(destination, asset)
					return err
				}
			}
			if output == "-" {
				return write(cmd.OutOrStdout())
			}
			if err := writeWikiDownload(output, overwrite, write); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Downloaded %s to %s (%s)\n", title, output, format)
			return err
		},
	}
	cmd.Flags().StringVar(&format, "format", "original", "Output format: original, png, or jpeg")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output path (default: wiki filename with the selected extension); - writes to stdout")
	cmd.Flags().IntVar(&width, "width", 0, "Exact converted width in pixels, preserving aspect ratio (0 uses the source width)")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace an existing output file")
	return cmd
}

func wikiDownloadSource(input string) (title, origin string, err error) {
	title = input
	if strings.Contains(input, "://") {
		u, parseErr := url.Parse(input)
		if parseErr != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
			return "", "", fmt.Errorf("use an HTTP or HTTPS file-description URL")
		}
		if strings.HasPrefix(u.Path, "/wiki/") {
			title = strings.TrimPrefix(u.Path, "/wiki/")
		} else if strings.HasSuffix(u.Path, "/index.php") {
			title = u.Query().Get("title")
		} else {
			return "", "", fmt.Errorf("use a file-description URL such as /wiki/File:Example.svg")
		}
		if !strings.Contains(title, ":") {
			return "", "", fmt.Errorf("URL must identify a file-description page")
		}
		origin = u.Scheme + "://" + u.Host
	} else if !strings.HasPrefix(strings.ToLower(title), "file:") && !strings.HasPrefix(strings.ToLower(title), "image:") {
		title = "File:" + title
	}
	if strings.TrimSpace(title) == "" || strings.TrimSpace(strings.SplitN(title, ":", 2)[1]) == "" || strings.ContainsAny(title, "|#\r\n\x00") {
		return "", "", fmt.Errorf("provide one nonempty wiki filename without fragments or separators")
	}
	return title, origin, nil
}

func queryWikiDownload(w *mwclient.Client, title string, width int) (wikiDownloadInfo, error) {
	p := params.Values{"action": "query", "formatversion": "2", "prop": "imageinfo", "titles": title, "redirects": "1", "iiprop": "url|mime|size|thumbmime"}
	if width > 0 {
		p["iiurlwidth"] = strconv.Itoa(width)
	}
	reply, err := w.Get(p)
	if err != nil {
		return wikiDownloadInfo{}, err
	}
	data, err := reply.Marshal()
	if err != nil {
		return wikiDownloadInfo{}, err
	}
	var result struct {
		Query struct {
			Pages []struct {
				Info []wikiDownloadInfo `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return wikiDownloadInfo{}, err
	}
	if len(result.Query.Pages) != 1 || len(result.Query.Pages[0].Info) != 1 || result.Query.Pages[0].Info[0].URL == "" {
		return wikiDownloadInfo{}, fmt.Errorf("no downloadable file found for %s", title)
	}
	return result.Query.Pages[0].Info[0], nil
}

func fetchWikiDownload(ctx context.Context, client *http.Client, rawURL, format string, destination io.Writer) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return fmt.Errorf("wiki returned an invalid download URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if format == "png" {
		req.Header.Set("Accept", "image/png")
	} else if format == "jpeg" {
		req.Header.Set("Accept", "image/jpeg")
	} else {
		req.Header.Set("Accept", "*/*")
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download file: HTTP %s", response.Status)
	}
	var source io.Reader = response.Body
	if format != "original" {
		source = io.LimitReader(source, maxConvertedImageBytes+1)
	}
	n, err := io.Copy(destination, source)
	if err != nil {
		return fmt.Errorf("download file: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("downloaded file is empty")
	}
	if format != "original" && n > maxConvertedImageBytes {
		return fmt.Errorf("preview is too large to convert; choose a smaller --width")
	}
	return nil
}

func decodeWikiDownload(source io.ReadSeeker, width int) (image.Image, error) {
	config, _, err := image.DecodeConfig(source)
	if err != nil {
		return nil, fmt.Errorf("cannot decode wiki preview (PNG, JPEG, GIF or WebP required): %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > maxConvertedImagePixels/int64(config.Height) {
		return nil, fmt.Errorf("preview dimensions are too large; choose a smaller --width")
	}
	height := max(1, int((int64(config.Height)*int64(width)+int64(config.Width)/2)/int64(config.Width)))
	if int64(width)*int64(height) > maxConvertedImagePixels {
		return nil, fmt.Errorf("converted dimensions are too large; choose a smaller --width")
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	decoded, _, err := image.Decode(source)
	if err != nil {
		return nil, fmt.Errorf("decode wiki preview: %w", err)
	}
	if decoded.Bounds().Dx() == width && decoded.Bounds().Dy() == height {
		return decoded, nil
	}
	resized := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), decoded, decoded.Bounds(), draw.Src, nil)
	return resized, nil
}

func writeWikiDownload(output string, overwrite bool, write func(io.Writer) error) error {
	temp, err := os.CreateTemp(filepath.Dir(output), ".mwcli-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := write(temp); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if overwrite {
		return os.Rename(temp.Name(), output)
	}
	if err := os.Link(temp.Name(), output); err != nil {
		return fmt.Errorf("save output %q (existing files require --overwrite): %w", output, err)
	}
	return nil
}
