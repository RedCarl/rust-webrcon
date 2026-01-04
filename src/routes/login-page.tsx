import * as React from "react";
import { useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { apiJson, getToken, setToken } from "@/lib/auth";
import { toast } from "sonner";

type LoginResponse = { token: string };
type RegisterResponse = { id: number; username: string };

export default function LoginPage() {
  const navigate = useNavigate();
  const [username, setUsername] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [loading, setLoading] = React.useState(false);

  React.useEffect(() => {
    if (getToken()) navigate("/servers", { replace: true });
  }, [navigate]);

  return (
    <div className="mx-auto flex min-h-dvh max-w-md items-center px-4 py-10">
      <Card className="w-full">
        <CardHeader>
          <CardTitle>登录</CardTitle>
          <CardDescription>登录后进入服务器列表</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor="username">用户名</Label>
            <Input
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              disabled={loading}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="password">密码</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              disabled={loading}
            />
          </div>
        </CardContent>
        <CardFooter>
          <div className="flex w-full gap-2">
            <Button
              className="flex-1"
              disabled={loading}
              onClick={async () => {
                setLoading(true);
                try {
                  const res = await apiJson<LoginResponse>("/api/auth/login", {
                    method: "POST",
                    body: JSON.stringify({ username, password }),
                  });
                  setToken(res.token);
                  navigate("/servers", { replace: true });
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "登录失败");
                } finally {
                  setLoading(false);
                }
              }}
            >
              {loading ? "登录中…" : "登录"}
            </Button>
            <Button
              className="flex-1"
              variant="secondary"
              disabled={loading}
              onClick={async () => {
                setLoading(true);
                try {
                  await apiJson<RegisterResponse>("/api/auth/register", {
                    method: "POST",
                    body: JSON.stringify({ username, password }),
                  });
                  const res = await apiJson<LoginResponse>("/api/auth/login", {
                    method: "POST",
                    body: JSON.stringify({ username, password }),
                  });
                  setToken(res.token);
                  navigate("/servers", { replace: true });
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : "注册失败");
                } finally {
                  setLoading(false);
                }
              }}
            >
              {loading ? "处理中…" : "注册"}
            </Button>
          </div>
        </CardFooter>
      </Card>
    </div>
  );
}
