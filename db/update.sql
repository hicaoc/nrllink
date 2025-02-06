ALTER TABLE devices ADD COLUMN callsign TEXT DEFAULT '';
ALTER TABLE public_groups ADD COLUMN allow_callsign_ssid TEXT DEFAULT '';
CREATE UNIQUE INDEX idx_ssid_callsign ON devices (ssid, callsign);
CREATE UNIQUE INDEX idx_name_unique ON public_groups(name);
CREATE UNIQUE INDEX idx_phone_unique ON users(phone);
CREATE UNIQUE INDEX idx_callsign_unique ON users(callsign);
