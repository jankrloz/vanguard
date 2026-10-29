import type { HistoryEntry, ScanResult, SystemStatus, UpgradeEvent } from "./types";

const BASE_URL = "";

export async function fetchStatus(): Promise<SystemStatus> {
  const res = await fetch(`${BASE_URL}/api/status`);
  if (!res.ok) throw new Error("Failed to fetch system status");
  return res.json();
}

export async function fetchOutdated(refresh = false): Promise<ScanResult> {
  const url = refresh ? `${BASE_URL}/api/outdated?refresh=true` : `${BASE_URL}/api/outdated`;
  const res = await fetch(url);
  if (!res.ok) {
    const errorData = await res.json().catch(() => ({}));
    throw new Error(errorData.error || "Failed to scan packages");
  }
  return res.json();
}

export async function triggerUpgrade(packages: string[]): Promise<{ status: string; count: number }> {
  const res = await fetch(`${BASE_URL}/api/upgrade`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ packages }),
  });
  if (!res.ok) {
    const errorText = await res.text();
    throw new Error(errorText || "Failed to trigger upgrade");
  }
  return res.json();
}

export async function triggerClean(): Promise<{ status: string }> {
  const res = await fetch(`${BASE_URL}/api/clean`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok) {
    const errorText = await res.text();
    throw new Error(errorText || "Failed to trigger clean");
  }
  return res.json();
}

export async function triggerTopgrade(): Promise<{ status: string }> {
  const res = await fetch(`${BASE_URL}/api/topgrade`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok) {
    const errorText = await res.text();
    throw new Error(errorText || "Failed to trigger Topgrade sweep");
  }
  return res.json();
}

export async function fetchHistory(limit = 50): Promise<HistoryEntry[]> {
  const res = await fetch(`${BASE_URL}/api/history?limit=${limit}`);
  if (!res.ok) throw new Error("Failed to fetch upgrade history");
  return res.json();
}

export async function togglePin(packageId: string): Promise<{ package_id: string; pinned: boolean }> {
  const res = await fetch(`${BASE_URL}/api/config/pin`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ package_id: packageId }),
  });
  if (!res.ok) {
    const errorText = await res.text();
    throw new Error(errorText || "Failed to toggle pin");
  }
  return res.json();
}

export function subscribeToEvents(onEvent: (event: UpgradeEvent) => void): () => void {
  const eventSource = new EventSource(`${BASE_URL}/api/events`);

  eventSource.onmessage = (e) => {
    try {
      const data: UpgradeEvent = JSON.parse(e.data);
      onEvent(data);
    } catch {
      // ignore parse err
    }
  };

  eventSource.onerror = () => {
    // EventSource will auto-reconnect
  };

  return () => {
    eventSource.close();
  };
}
