package main

import "testing"

func TestLoadMediaConfigAllowsDisabledDevelopment(t *testing.T) {
	config, err := loadMediaConfig(func(string) string { return "" })
	if err != nil || config.enabled {
		t.Fatalf("loadMediaConfig() = %#v, %v", config, err)
	}
}

func TestLoadMediaConfigRequiresCompleteHTTPSConfiguration(t *testing.T) {
	values := map[string]string{
		"MEDIA_COS_BUCKET_URL":  "https://bucket.cos.ap-nanjing.myqcloud.com",
		"MEDIA_PUBLIC_BASE_URL": "https://media.example.com",
		"MEDIA_COS_SECRET_ID":   "secret-id",
		"MEDIA_COS_SECRET_KEY":  "secret-key",
	}
	config, err := loadMediaConfig(func(key string) string { return values[key] })
	if err != nil || !config.enabled {
		t.Fatalf("loadMediaConfig() = %#v, %v", config, err)
	}

	delete(values, "MEDIA_COS_SECRET_KEY")
	if _, err := loadMediaConfig(func(key string) string { return values[key] }); err == nil {
		t.Fatal("loadMediaConfig() accepted partial configuration")
	}

	values["MEDIA_COS_SECRET_KEY"] = "secret-key"
	values["MEDIA_PUBLIC_BASE_URL"] = "https://media.example.com?variant=unsafe"
	if _, err := loadMediaConfig(func(key string) string { return values[key] }); err == nil {
		t.Fatal("loadMediaConfig() accepted a public URL with a query")
	}
}
