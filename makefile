######################################################################
## Copyright (C) Ellem, Inc. - All Rights Reserved                  ##
## Unauthorized copying of this content is strictly prohibited.     ##
######################################################################
#GLOBAL
PRODUCT      = doccomp
VERSION     := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "0.0.0")
MAKEDIR      = .
BUILD_DIR    = $(MAKEDIR)/build
MAKEFILE_REVISION = 3
default: all


#*********************************************************************
#
# go targets
#
#*********************************************************************

GOC           = go
APISRC        = $(MAKEDIR)/go
GO_FILES     := $(wildcard $(APISRC)/*.go)
GO_FILES_ALL := $(shell find $(APISRC) -name \*.go -type f)
GO_BIN        = $(BUILD_DIR)/$(PRODUCT)
GO_BIN_INSTALL=$(DESTDIR)/usr/bin/$(PRODUCT)
OPERATION_VAR_DIRS=$(DESTDIR)/var/lib/$(PRODUCT)

$(GO_BIN): $(GO_FILES_ALL)
	$(GOC) build -o $(GO_BIN) $(GO_FILES)

$(GO_BIN_INSTALL):$(GO_BIN)
	install --strip $(GO_BIN) -DT $(GO_BIN_INSTALL)

gosrc:$(GO_BIN)
gosrc-test: $(GO_FILES) $(GO_FILE_ALL)
	$(GOC) test $(GO_FILES)

gosrc-install:$(GO_BIN_INSTALL)
	mkdir -p $(OPERATION_VAR_DIRS)

gosrc-remove:
	-rm $(GO_BIN_INSTALL)
	-rmdir $(OPERATION_VAR_DIRS)

gosrc-clean:
	-rm $(GO_BIN)




#*********************************************************************
#
# misc targets
#
#*********************************************************************


######## config ########

SERVICE_INSTALL = $(DESTDIR)/lib/systemd/system/$(PRODUCT).service
CONF_INSTALL    = $(DESTDIR)/etc/$(PRODUCT)/$(PRODUCT).conf

$(SERVICE_INSTALL): config/$(PRODUCT).service
	install -m644 -D $< $@

$(CONF_INSTALL): config/$(PRODUCT).conf
	install -m644 -D $< $@


config: config/$(PRODUCT).nginx
config-install:$(SERVICE_INSTALL) $(CONF_INSTALL)
config-remove: 
	-rm $(NGINX_INSTALL)
	-rm $(SERVICE_INSTALL)
config-test:
config-clean:


######## /usr/share/doc ########

DOC_FILES     = doc/readme.org
COPYRIGHT_INSTALL = $(DESTDIR)/usr/share/doc/$(PRODUCT)/copyright
ORG_INSTALL = $(DESTDIR)/usr/share/doc/$(PRODUCT)/readme.org
MAN_INSTALL = $(DESTDIR)/usr/share/man/man1/$(PRODUCT).1.gz

$(MAN_INSTALL): doc/$(PRODUCT).man
	mkdir -p `dirname $(MAN_INSTALL)`
	gzip -9 -n -c $< > $@

$(ORG_INSTALL): doc/readme.org
	mkdir -p `dirname $(ORG_INSTALL)`
	cp doc/readme.org $(ORG_INSTALL)

$(COPYRIGHT_INSTALL): doc/copyright
	install -m0644 -D $< $@

######## all of the above will be in the below targets ########

doc: 
doc-install: $(ORG_INSTALL) $(MAN_INSTALL) $(COPYRIGHT_INSTALL)
doc-test: 
doc-clean:
doc-remove:
	-rm $(ORG_INSTALL)
	-rm $(MAN_INSTALL)
	-rm $(COPYRIGHT_INSTALL)


#*********************************************************************
#
# targets
#
#*********************************************************************


######### System targets ########

# default... what to run without any commands
MASTER_TARGETS   = gosrc doc config
INSTALL_TARGETS := $(patsubst %, %-install, $(MASTER_TARGETS))
REMOVE_TARGETS := $(patsubst %, %-uninstall, $(MASTER_TARGETS))
CLEAN_TARGETS := $(patsubst %, %-clean, $(MASTER_TARGETS))
TEST_TARGETS := $(patsubst %, %-test, $(MASTER_TARGETS))

.PHONY: clean install default build remove help all .force $(MASTER_TARGETS) $(INSTALL_TARGETS) $(REMOVE_TARGETS) $(CLEAN_TARGETS) $(TEST_TARGETS)
help:
	@echo "make [VARIABLE=VALUE]... [TARGET]"
	@echo  "\nHigh level targets:"
	@echo  "  all        - build everything that needs to be (default target)"
	@echo  "  install    - install the files into the system (invokes build all)"
	@echo  "  remove     - uninstall from the system (errors ignored)"
	@echo  "  test       - test the built product (invokes build all)"
	@echo  "\nHigh level variables:"
	@echo  "  DESTDIR - prepended to each installed target file, useful for packaging"

install: $(INSTALL_TARGETS)

remove: $(REMOVE_TARGETS)

test: $(TEST_TARGETS)

all: $(MASTER_TARGETS)

clean: $(CLEAN_TARGETS)
