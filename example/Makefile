# Gotzer Makefile for Example Project

# Path to the gotzer binary
GOTZER ?= ../gotzer

.PHONY: all
all: deploy

## -- Infrastructure Management --

# Create the server and install basic tools (Docker, etc)
provision:
	$(GOTZER) provision

# Destroy the server and all data permanently
destroy:
	$(GOTZER) destroy

# Show the current status of the server and services
status:
	$(GOTZER) status

## -- Application Management --

# Build and deploy the Go application (default)
deploy:
	$(GOTZER) deploy

# SSH into the server for manual management
ssh:
	$(GOTZER) ssh

# Follow application logs
logs:
	$(GOTZER) logs -f

## -- Local Setup --

# Initialize a new project configuration
init:
	$(GOTZER) init

# Configure Hetzner API token
auth:
	$(GOTZER) auth

## -- Help --
help:
	@echo "Gotzer Deployment Commands:"
	@echo "  make provision  - Create new server on Hetzner"
	@echo "  make deploy     - Build and deploy Go app"
	@echo "  make status     - Show server/app status"
	@echo "  make ssh        - SSH into the server"
	@echo "  make logs       - View live app logs"
	@echo "  make destroy    - Delete the server (WARNING!)"
	@echo "  make auth       - Set Hetzner API token"
