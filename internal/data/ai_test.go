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

func TestDeleteProviderKeepsSelectionByID(t *testing.T) {
	c := DefaultConfig()
	c.SetProviders([]AIProvider{
		{Name: "A", BaseURL: "https://a", Model: "a"},
		{Name: "B", BaseURL: "https://b", Model: "b"},
		{Name: "C", BaseURL: "https://c", Model: "c"},
		{Name: "D", BaseURL: "https://d", Model: "d"},
	}, 2)
	c.EnsureProviders()

	selected := c.Providers[c.ActiveProvider]
	if selected.Name != "C" {
		t.Fatalf("setup: active = %q", selected.Name)
	}
	c.DeleteProvider(0) // 删除 A
	if got := c.Providers[c.ActiveProvider]; got.ProviderID() != selected.ProviderID() || got.Name != "C" {
		t.Fatalf("删除前面的项后应仍选中 C，实际 %q", got.Name)
	}
}

func TestProviderIDsAssignedAndStable(t *testing.T) {
	c := DefaultConfig()
	c.SetProviders([]AIProvider{
		{Name: "A", BaseURL: "https://a", Model: "a"},
		{Name: "A", BaseURL: "https://a2", Model: "a"},
	}, 0)
	c.EnsureProviders()
	if c.Providers[0].ID == "" || c.Providers[1].ID == "" {
		t.Fatal("providers should get stable IDs")
	}
	if c.Providers[0].ID == c.Providers[1].ID {
		t.Fatal("provider IDs must be unique")
	}
}
