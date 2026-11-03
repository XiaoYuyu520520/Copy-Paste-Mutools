package data

import "testing"

func TestEnsureProvidersMigratesLegacyAI(t *testing.T) {
	c := DefaultConfig()
	c.AI = &AIConfig{BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat", APIKey: "k"}
	c.EnsureProviders()
	if len(c.Providers) != 1 {
		t.Fatalf("providers = %d", len(c.Providers))
	}
	if c.Providers[0].BaseURL != c.AI.BaseURL || c.Providers[0].Model != "deepseek-chat" {
		t.Fatalf("migration mismatch: %+v", c.Providers[0])
	}
}

func TestProviderCRUDAndActiveSync(t *testing.T) {
	c := DefaultConfig()
	c.EnsureProviders()

	c.UpsertProvider(-1, AIProvider{Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", Model: "deepseek-chat"})
	if c.ActiveProvider != 1 || c.AI.Model != "deepseek-chat" {
		t.Fatalf("upsert did not activate: %+v", c)
	}

	c.SelectProvider(0)
	if c.AI.Model == "deepseek-chat" {
		t.Fatal("select provider did not sync AI")
	}

	c.UpsertProvider(0, AIProvider{Name: "OpenAI", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"})
	if c.Providers[0].Model != "gpt-4o-mini" || c.AI.Model != "gpt-4o-mini" {
		t.Fatalf("update failed: %+v", c)
	}

	c.DeleteProvider(1)
	if len(c.Providers) != 1 {
		t.Fatalf("delete failed: %d", len(c.Providers))
	}
	c.DeleteProvider(0)
	if len(c.Providers) != 1 {
		t.Fatal("must keep at least one provider")
	}
}
