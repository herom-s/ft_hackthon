locals {
  cloud_init = base64encode(<<-EOF
    #cloud-config
    package_update: true
    packages:
      - git
      - docker.io
      - docker-compose-v2
    runcmd:
      - systemctl enable --now docker
      - git clone ${var.repo_url} /opt/ft_hackthon
      - cd /opt/ft_hackthon
      - cp .env.example .env
      - PUBLIC_IP=$(curl -s http://ifconfig.me)
      - sed -i "s|GITEA_PUBLIC_URL=.*|GITEA_PUBLIC_URL=http://$PUBLIC_IP:3222|" .env
      - sed -i "s|^DOMAIN=.*|DOMAIN=|" .env
      - |
        if [ -n "${var.domain}" ]; then
          sed -i "s|^DOMAIN=.*|DOMAIN=${var.domain}|" .env
          sed -i "s|^GITEA_PUBLIC_URL=.*|GITEA_PUBLIC_URL=http://${var.domain}:3222|" .env
        fi
      - docker compose up -d --build
  EOF
  )
}

resource "azurerm_resource_group" "this" {
  name     = "ft-hackthon"
  location = var.region
}

# Rules are inline so the NSG is written in one request: separate
# azurerm_network_security_rule resources update the same NSG concurrently
# and intermittently fail.
resource "azurerm_network_security_group" "this" {
  name                = "ft-hackthon"
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name

  dynamic "security_rule" {
    for_each = ["22", "80", "8443", "3222"]
    content {
      name                       = "Allow-${security_rule.value}"
      priority                   = 100 + security_rule.key
      direction                  = "Inbound"
      access                     = "Allow"
      protocol                   = "Tcp"
      source_port_range          = "*"
      destination_port_range     = security_rule.value
      source_address_prefix      = "*"
      destination_address_prefix = "*"
    }
  }
}

resource "azurerm_virtual_network" "this" {
  name                = "ft-hackthon-vnet"
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  address_space       = ["10.0.0.0/16"]
}

resource "azurerm_subnet" "this" {
  name                 = "default"
  resource_group_name  = azurerm_resource_group.this.name
  virtual_network_name = azurerm_virtual_network.this.name
  address_prefixes     = ["10.0.1.0/24"]
}

resource "azurerm_subnet_network_security_group_association" "this" {
  subnet_id                 = azurerm_subnet.this.id
  network_security_group_id = azurerm_network_security_group.this.id
}

resource "azurerm_public_ip" "this" {
  name                = "ft-hackthon-ip"
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  allocation_method   = "Static"
  sku                 = "Standard"
}

resource "azurerm_network_interface" "this" {
  name                = "ft-hackthon-nic"
  location            = azurerm_resource_group.this.location
  resource_group_name = azurerm_resource_group.this.name
  ip_configuration {
    name                          = "primary"
    subnet_id                     = azurerm_subnet.this.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.this.id
  }
}

resource "azurerm_linux_virtual_machine" "this" {
  name                  = "ft-hackthon"
  location              = azurerm_resource_group.this.location
  resource_group_name   = azurerm_resource_group.this.name
  size                  = var.size
  admin_username        = "ftuser"
  custom_data           = local.cloud_init
  network_interface_ids = [azurerm_network_interface.this.id]
  admin_ssh_key {
    username   = "ftuser"
    public_key = var.ssh_public_key
  }
  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
  }
  source_image_reference {
    publisher = "Canonical"
    offer     = "ubuntu-24_04-lts"
    sku       = "server"
    version   = "latest"
  }
}

output "ip" {
  value = azurerm_public_ip.this.ip_address
}

output "ssh_command" {
  value = "ssh ftuser@${azurerm_public_ip.this.ip_address}"
}

output "api_url" {
  value = "https://${azurerm_public_ip.this.ip_address}:8443/api/v1"
}
