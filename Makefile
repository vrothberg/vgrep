TARGET = vgrep
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

install:
	install -D $(TARGET) $(BINDIR)

uninstall:
	-rm -f $(BINDIR)/$(TARGET)

.PHONY: install uninstall
