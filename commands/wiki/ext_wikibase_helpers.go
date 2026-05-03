package wiki

import (
	"encoding/json"
	"fmt"

	mwclient "cgt.name/pkg/go-mwclient"
	"cgt.name/pkg/go-mwclient/params"
)

func wbEditEntity(w *mwclient.Client, p params.Values) (string, error) {
	if p["token"] == "" {
		token, err := w.GetToken(mwclient.CSRFToken)
		if err != nil {
			return "", fmt.Errorf("unable to obtain csrf token: %w", err)
		}
		p["token"] = token
	}

	p["action"] = "wbeditentity"
	resp, err := w.Post(p)
	if err != nil {
		return "", err
	}

	id, err := resp.GetString("entity", "id")
	if err == nil && id != "" {
		return id, nil
	}

	raw, marshalErr := resp.Marshal()
	if marshalErr == nil {
		return "", fmt.Errorf("wbeditentity response missing entity.id: %s", string(raw))
	}
	return "", fmt.Errorf("wbeditentity response missing entity.id")
}

func mustMarshalJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
