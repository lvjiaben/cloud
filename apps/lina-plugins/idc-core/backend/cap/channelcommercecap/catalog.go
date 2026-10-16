// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package channelcommercecap

import "context"

// CatalogConfigurationRuntime exposes the same safe, opaque configuration
// contract used by the storefront. Preview amounts are not customer quotes.
type CatalogConfigurationRuntime interface {
	BrowseCatalogNumbered(context.Context, CatalogNumberedQuery) (*CatalogNumberedPage, error)
	ReadProductSnapshot(context.Context, string) (*ProductSnapshot, error)
	ReadCatalogConfiguration(context.Context, string) (*Configuration, error)
	EvaluateCatalogConfiguration(context.Context, ConfigurationInput) (*ConfigurationEvaluation, error)
	PreviewCatalogTerms(context.Context, ConfigurationInput) (*BillingTerms, error)
}

type CatalogNumberedQuery struct {
	NamePrefix, Currency string
	Page, PageSize       int
}
type CatalogNumberedPage struct {
	Products []CatalogProduct `json:"products"`
	Total    int              `json:"total"`
}

type LocalizedText struct {
	ZhCN string `json:"zhCn"`
	EnUS string `json:"enUs"`
}

type ConfigurationOption struct {
	OptionToken string                `json:"optionToken"`
	Label       LocalizedText         `json:"label"`
	Children    []ConfigurationOption `json:"children"`
}

type ConfigurationConstraints struct {
	Format        string `json:"format"`
	Minimum       string `json:"minimum"`
	Maximum       string `json:"maximum"`
	MinimumLength int    `json:"minimumLength"`
	MaximumLength int    `json:"maximumLength"`
}

type ConfigurationUploadPolicy struct {
	MaximumFiles       int      `json:"maximumFiles"`
	MaximumFileBytes   int64    `json:"maximumFileBytes"`
	MaximumTotalBytes  int64    `json:"maximumTotalBytes"`
	AllowedMIMETypes   []string `json:"allowedMimeTypes"`
	AllowedExtensions  []string `json:"allowedExtensions"`
	SafetyScanRequired bool     `json:"safetyScanRequired"`
}

type ConfigurationField struct {
	FieldToken   string                     `json:"fieldToken"`
	Type         string                     `json:"type"`
	Label        LocalizedText              `json:"label"`
	Description  LocalizedText              `json:"description"`
	Visible      bool                       `json:"visible"`
	Required     bool                       `json:"required"`
	Disabled     bool                       `json:"disabled"`
	Constraints  ConfigurationConstraints   `json:"constraints"`
	Options      []ConfigurationOption      `json:"options"`
	UploadPolicy *ConfigurationUploadPolicy `json:"uploadPolicy"`
}

type Configuration struct {
	ConfigurationRevision string               `json:"configurationRevision"`
	Fields                []ConfigurationField `json:"fields"`
	Values                []PurchaseValue      `json:"values"`
}

type ConfigurationInput struct {
	CustomerNo            string          `json:"-"`
	PublicSlug            string          `json:"publicSlug"`
	ConfigurationRevision string          `json:"configurationRevision"`
	CurrencyCode          string          `json:"currencyCode"`
	Values                []PurchaseValue `json:"values"`
}

type ConfigurationIssue struct {
	FieldToken string `json:"fieldToken"`
	IssueCode  string `json:"issueCode"`
}

type ConfigurationEvaluation struct {
	Configuration
	Valid  bool                 `json:"valid"`
	Issues []ConfigurationIssue `json:"issues"`
}

type BillingTerm struct {
	BillingTermToken      string        `json:"billingTermToken"`
	Label                 LocalizedText `json:"label"`
	ChargeType            string        `json:"chargeType"`
	PeriodUnit            string        `json:"periodUnit"`
	PeriodCount           int           `json:"periodCount"`
	MinimumPurchaseCycles int           `json:"minimumPurchaseCycles"`
	Status                string        `json:"status"`
	SetupAmount           string        `json:"setupAmount"`
	SubtotalAmount        string        `json:"subtotalAmount"`
	TaxAmount             string        `json:"taxAmount"`
	TotalAmount           string        `json:"totalAmount"`
}

type BillingTerms struct {
	ConfigurationRevision string        `json:"configurationRevision"`
	CurrencyCode          string        `json:"currencyCode"`
	Terms                 []BillingTerm `json:"terms"`
}
