PREFIX      ?= /opt/frinklip
CONFIG_DIR  ?= /etc/frinklip
DROP_DIR    ?= /tmp/dropped
USER_NAME   ?= filedrop
UNIT        ?= /etc/systemd/system/frinklip.service

GO          ?= go
BIN         := frinklip
LDFLAGS     := -s -w

.PHONY: build clean install uninstall user dirs deploy restart status logs

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN) ./cmd/frinklip

clean:
	rm -f $(BIN)

user:
	id -u $(USER_NAME) >/dev/null 2>&1 || \
	  sudo useradd --system --no-create-home --shell /usr/sbin/nologin $(USER_NAME)

dirs:
	sudo install -d -o $(USER_NAME) -g $(USER_NAME) -m 0755 $(DROP_DIR)
	sudo install -d -m 0755 $(PREFIX) $(CONFIG_DIR)

deploy: build user dirs
	sudo install -m 0755 $(BIN) $(PREFIX)/$(BIN)
	sudo install -m 0644 systemd/frinklip.yaml $(CONFIG_DIR)/frinklip.yaml
	sudo install -m 0644 systemd/frinklip.service $(UNIT)
	sudo systemctl daemon-reload

install: deploy
	sudo systemctl enable --now frinklip
	@echo
	@echo "frinklip running. Try: curl -s http://127.0.0.1/ | head -5"

restart:
	sudo systemctl restart frinklip

status:
	systemctl status frinklip --no-pager -l | head -30

logs:
	journalctl -u frinklip -n 80 --no-pager

uninstall:
	-sudo systemctl disable --now frinklip
	sudo rm -f $(UNIT) $(PREFIX)/$(BIN) $(CONFIG_DIR)/frinklip.yaml
	-sudo rmdir $(PREFIX) $(CONFIG_DIR) 2>/dev/null
	sudo systemctl daemon-reload
	@echo "user '$(USER_NAME)' and $(DROP_DIR) left intact"
