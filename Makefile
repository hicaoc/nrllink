include $(TOPDIR)/rules.mk

PKG_NAME:=NRLLINK
PKG_VERSION:=2
PKG_RELEASE:=2

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)

include $(INCLUDE_DIR)/package.mk

define Package/nrllink
  SECTION:=network
  CATEGORY:=radius
  TITLE:=NRLLink
  DEPENDS:=
endef

define Package/<插件名称>/description
  NRL协议的服务器程序，用于调度无线电语言，控制信令，通过网络互联无线电设备(中继台，手台，等)
endef

define Build/Prepare
  mkdir -p $(PKG_BUILD_DIR)
  cp -R ./src/* $(PKG_BUILD_DIR)/
endef

define Build/Compile
  $(MAKE) -C $(PKG_BUILD_DIR) all
endef

define Package/nrllink/install
  $(INSTALL_DIR) $(1)/usr/bin
  $(INSTALL_BIN) $(PKG_BUILD_DIR)/nrllink $(1)/usr/bin/
endef

$(eval $(call BuildPackage,nrllink))