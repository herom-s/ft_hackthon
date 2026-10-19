provider "azurerm" {
  resource_provider_registrations = "none"
  use_cli                         = false

  environment   = "stack"
  metadata_host = var.floci_metadata_host

  subscription_id = "00000000-0000-0000-0000-000000000001"
  tenant_id       = "00000000-0000-0000-0000-000000000002"
  client_id       = "00000000-0000-0000-0000-000000000003"
  client_secret   = "fake-secret"
}
