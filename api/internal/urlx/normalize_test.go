package urlx

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{
			name: "strips utm and fragment",
			in:   "https://WWW.Example.com/product/abc/?utm_source=news&utm_campaign=x&color=red#section",
			want: "https://www.example.com/product/abc?color=red",
		},
		{
			name: "strips trailing slash",
			in:   "https://shop.example/item/1/",
			want: "https://shop.example/item/1",
		},
		{
			name: "keeps meaningful query params sorted",
			in:   "https://shop.example/p?b=2&a=1",
			want: "https://shop.example/p?a=1&b=2",
		},
		{
			name:    "rejects relative",
			in:      "/just/a/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStoreHost(t *testing.T) {
	if got := StoreHost("https://www.ikea.com/se/sv/p/x"); got != "ikea.com" {
		t.Fatalf("got %q", got)
	}
}
