import * as React from "react";
import { useNavigate } from "react-router-dom";
import { RefreshCw, Save } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { apiJson, clearToken } from "@/lib/auth";
import { toast } from "sonner";

type WipeConfigDto = {
  lgsm_dir?: string;
  common_cfg?: string;
  map_dir?: string;
  server_identity?: string;
  rustmaps_search_url?: string;
  rustmaps_search_body?: string;
  s3_endpoint?: string;
  s3_region?: string;
  s3_bucket?: string;
  s3_access_key?: string;
  s3_secret_key?: string;
  public_base_url?: string;
  web_rcon_api_url?: string;
  web_rcon_server_id?: number;
  map_detect_timeout_seconds?: number;
};

type AdminWipeSettingsResponse = {
  wipe_config?: WipeConfigDto | null;
};

type WipeConfigForm = {
  lgsmDir: string;
  commonCfg: string;
  mapDir: string;
  serverIdentity: string;
  rustMapsSearchURL: string;
  rustMapsSearchBody: string;
  s3Endpoint: string;
  s3Region: string;
  s3Bucket: string;
  s3AccessKey: string;
  s3SecretKey: string;
  publicBaseURL: string;
  webRconApiURL: string;
  webRconServerID: string;
  mapDetectTimeoutSeconds: string;
};

function defaultRustMapsSearchBody() {
  return JSON.stringify({}, null, 2);
}

function defaultForm(): WipeConfigForm {
  return {
    lgsmDir: "/data/serverfiles",
    commonCfg: "/data/config-lgsm/rustserver/common.cfg",
    mapDir: "/data/serverfiles/server/rustserver",
    serverIdentity: "rustserver",
    rustMapsSearchURL: "https://api.rustmaps.com/internal/v1/maps/search",
    rustMapsSearchBody: defaultRustMapsSearchBody(),
    s3Endpoint: "",
    s3Region: "auto",
    s3Bucket: "",
    s3AccessKey: "",
    s3SecretKey: "",
    publicBaseURL: "",
    webRconApiURL: "https://rust-webrcon.carljoy.cn",
    webRconServerID: "",
    mapDetectTimeoutSeconds: "300",
  };
}

function formFromDto(dto?: WipeConfigDto | null): WipeConfigForm {
  const base = defaultForm();
  if (!dto) return base;
  let rustMapsSearchBody = base.rustMapsSearchBody;
  if (dto.rustmaps_search_body) {
    try {
      rustMapsSearchBody = JSON.stringify(JSON.parse(dto.rustmaps_search_body), null, 2);
    } catch {
      rustMapsSearchBody = dto.rustmaps_search_body;
    }
  }
  return {
    lgsmDir: dto.lgsm_dir ?? base.lgsmDir,
    commonCfg: dto.common_cfg ?? base.commonCfg,
    mapDir: dto.map_dir ?? base.mapDir,
    serverIdentity: dto.server_identity ?? base.serverIdentity,
    rustMapsSearchURL: dto.rustmaps_search_url ?? base.rustMapsSearchURL,
    rustMapsSearchBody: rustMapsSearchBody,
    s3Endpoint: dto.s3_endpoint ?? base.s3Endpoint,
    s3Region: dto.s3_region ?? base.s3Region,
    s3Bucket: dto.s3_bucket ?? base.s3Bucket,
    s3AccessKey: dto.s3_access_key ?? base.s3AccessKey,
    s3SecretKey: "",
    publicBaseURL: dto.public_base_url ?? base.publicBaseURL,
    webRconApiURL: dto.web_rcon_api_url ?? base.webRconApiURL,
    webRconServerID: dto.web_rcon_server_id ? String(dto.web_rcon_server_id) : "",
    mapDetectTimeoutSeconds: dto.map_detect_timeout_seconds ? String(dto.map_detect_timeout_seconds) : base.mapDetectTimeoutSeconds,
  };
}

function dtoFromForm(form: WipeConfigForm): WipeConfigDto {
  const webRconServerIDText = form.webRconServerID.trim();
  const timeoutText = form.mapDetectTimeoutSeconds.trim();
  const rustMapsSearchBodyText = form.rustMapsSearchBody.trim();

  if (!rustMapsSearchBodyText) {
    throw new Error("RustMaps 搜索条件不能为空");
  }
  const parsedBody = JSON.parse(rustMapsSearchBodyText);
  if (!parsedBody || typeof parsedBody !== "object" || Array.isArray(parsedBody)) {
    throw new Error("RustMaps 搜索条件必须是 JSON 对象");
  }

  const out: WipeConfigDto = {
    lgsm_dir: form.lgsmDir.trim(),
    common_cfg: form.commonCfg.trim(),
    map_dir: form.mapDir.trim(),
    server_identity: form.serverIdentity.trim(),
    rustmaps_search_url: form.rustMapsSearchURL.trim(),
    rustmaps_search_body: JSON.stringify(parsedBody),
    s3_endpoint: form.s3Endpoint.trim(),
    s3_region: form.s3Region.trim(),
    s3_bucket: form.s3Bucket.trim(),
    s3_access_key: form.s3AccessKey.trim(),
    public_base_url: form.publicBaseURL.trim(),
    web_rcon_api_url: form.webRconApiURL.trim(),
  };

  if (form.s3SecretKey.trim()) {
    out.s3_secret_key = form.s3SecretKey.trim();
  }
  if (webRconServerIDText) {
    const value = Number(webRconServerIDText);
    if (!Number.isFinite(value) || !Number.isInteger(value) || value <= 0) {
      throw new Error("WebRCON Server ID 不合法");
    }
    out.web_rcon_server_id = value;
  }
  if (timeoutText) {
    const value = Number(timeoutText);
    if (!Number.isFinite(value) || !Number.isInteger(value) || value <= 0) {
      throw new Error("地图检测超时时间不合法");
    }
    out.map_detect_timeout_seconds = value;
  }
  return out;
}

export default function SettingsWipePage() {
  const navigate = useNavigate();
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<WipeConfigForm>(defaultForm());

  const setField = React.useCallback((key: keyof WipeConfigForm, value: string) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  }, []);

  const refresh = React.useCallback(async () => {
    setLoading(true);
    try {
      const data = await apiJson<AdminWipeSettingsResponse>("/api/admin/settings/wipe");
      setForm(formFromDto(data?.wipe_config));
    } catch (e) {
      const msg = e instanceof Error ? e.message : "加载失败";
      toast.error(msg);
      if (msg.includes("未登录")) {
        clearToken();
        navigate("/login", { replace: true });
      }
    } finally {
      setLoading(false);
    }
  }, [navigate]);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0">
        <div className="grid gap-1.5">
          <CardTitle>容器配置</CardTitle>
          <CardDescription>这里主要配置 LinuxGSM、地图文件、S3 和 WebRCON。不是整块 JSON 编辑。</CardDescription>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="icon" aria-label="刷新" disabled={loading || saving} onClick={refresh}>
            <RefreshCw className="h-4 w-4" />
          </Button>
          <Button
            size="icon"
            aria-label="保存"
            disabled={loading || saving}
            onClick={async () => {
              setSaving(true);
              try {
                const wipeConfig = dtoFromForm(form);
                await apiJson<AdminWipeSettingsResponse>("/api/admin/settings/wipe", {
                  method: "PUT",
                  body: JSON.stringify({ wipe_config: wipeConfig }),
                });
                toast.success("已保存");
                await refresh();
              } catch (e) {
                toast.error(e instanceof Error ? e.message : "保存失败");
              } finally {
                setSaving(false);
              }
            }}
          >
            <Save className="h-4 w-4" />
          </Button>
        </div>
      </CardHeader>
      <CardContent className="grid gap-6">
        <section className="grid gap-4">
          <div>
            <div className="text-sm font-medium">容器与 LinuxGSM</div>
            <div className="text-xs text-muted-foreground">这些路径和标识会用于容器内执行 LinuxGSM 相关操作。</div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="lgsm-dir">LinuxGSM 目录</Label>
              <Input id="lgsm-dir" value={form.lgsmDir} onChange={(e) => setField("lgsmDir", e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="common-cfg">common.cfg 路径</Label>
              <Input id="common-cfg" value={form.commonCfg} onChange={(e) => setField("commonCfg", e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="map-dir">地图目录</Label>
              <Input id="map-dir" value={form.mapDir} onChange={(e) => setField("mapDir", e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="server-identity">服务器标识</Label>
              <Input
                id="server-identity"
                value={form.serverIdentity}
                onChange={(e) => setField("serverIdentity", e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="map-detect-timeout">地图检测超时（秒）</Label>
              <Input
                id="map-detect-timeout"
                type="number"
                value={form.mapDetectTimeoutSeconds}
                onChange={(e) => setField("mapDetectTimeoutSeconds", e.target.value)}
              />
            </div>
          </div>
        </section>

        <section className="grid gap-4">
          <div>
            <div className="text-sm font-medium">RustMaps</div>
            <div className="text-xs text-muted-foreground">筛选条件本身仍然是 JSON，但只保留为单独字段，不再整页 JSON 编辑。</div>
          </div>
          <div className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="rustmaps-url">RustMaps 搜索地址</Label>
              <Input
                id="rustmaps-url"
                value={form.rustMapsSearchURL}
                onChange={(e) => setField("rustMapsSearchURL", e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="rustmaps-body">RustMaps 搜索条件 JSON</Label>
              <Textarea
                id="rustmaps-body"
                className="min-h-40 font-mono text-xs"
                value={form.rustMapsSearchBody}
                onChange={(e) => setField("rustMapsSearchBody", e.target.value)}
                spellCheck={false}
              />
            </div>
          </div>
        </section>

        <section className="grid gap-4">
          <div>
            <div className="text-sm font-medium">S3 与下载地址</div>
            <div className="text-xs text-muted-foreground">`S3 Secret` 留空时保持原值不变。</div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="s3-endpoint">S3 Endpoint</Label>
              <Input id="s3-endpoint" value={form.s3Endpoint} onChange={(e) => setField("s3Endpoint", e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="s3-region">S3 Region</Label>
              <Input id="s3-region" value={form.s3Region} onChange={(e) => setField("s3Region", e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="s3-bucket">S3 Bucket</Label>
              <Input id="s3-bucket" value={form.s3Bucket} onChange={(e) => setField("s3Bucket", e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="s3-access-key">S3 Access Key</Label>
              <Input
                id="s3-access-key"
                value={form.s3AccessKey}
                onChange={(e) => setField("s3AccessKey", e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="s3-secret-key">S3 Secret</Label>
              <Input
                id="s3-secret-key"
                type="password"
                value={form.s3SecretKey}
                onChange={(e) => setField("s3SecretKey", e.target.value)}
                placeholder="留空则保持原值"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="public-base-url">地图公网地址前缀</Label>
              <Input
                id="public-base-url"
                value={form.publicBaseURL}
                onChange={(e) => setField("publicBaseURL", e.target.value)}
              />
            </div>
          </div>
        </section>

        <section className="grid gap-4">
          <div>
            <div className="text-sm font-medium">WebRCON</div>
            <div className="text-xs text-muted-foreground">清档完成后会用这里的配置同步运行时 `levelurl`。</div>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="web-rcon-api-url">WebRCON API 地址</Label>
              <Input
                id="web-rcon-api-url"
                value={form.webRconApiURL}
                onChange={(e) => setField("webRconApiURL", e.target.value)}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="web-rcon-server-id">WebRCON Server ID</Label>
              <Input
                id="web-rcon-server-id"
                type="number"
                value={form.webRconServerID}
                onChange={(e) => setField("webRconServerID", e.target.value)}
              />
            </div>
          </div>
        </section>
      </CardContent>
    </Card>
  );
}
