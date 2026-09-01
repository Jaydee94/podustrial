# Podustrial — Technische Architektur

**Status:** Entwurf, abgestimmt in Brainstorming-Session am 2026-09-01.
**Scope:** Nur die technische Grundarchitektur. Konkrete Spielmechanik pro Level, Gewinnbedingungen, Metapher-Grenzen und Arbeitsaufteilung sind bewusst nicht Teil dieser Spec (siehe Abschnitt 8).

Grundlage: `podustrial-konzept.md` (Grundidee, Metapher-Zuordnung, didaktische Prinzipien, Lernkurve).

---

## 1. Zielbild

Podustrial läuft zunächst als **lokale Single-Player-Anwendung**: Der Spieler startet das Spiel auf dem eigenen Rechner, ein echter [kind](https://kind.sigs.k8s.io/)-Kubernetes-Cluster wird lokal erzeugt, das Spiel läuft im Browser gegen `localhost`. Ein späterer Modus mit geteiltem Cluster für mehrere Spieler ist als Erweiterungsrichtung mitgedacht, aber nicht Teil dieses Designs.

Kernentscheidung: Spieleraktionen erzeugen **echte Kubernetes-Objekte** (Deployments, Pods mit Dummy-Containern wie `busybox sleep`) im echten kind-Cluster. Scheduling und Self-Healing kommen vom echten K8s-Scheduler/Controller, nicht von simulierter Backend-Logik. Das Backend ist Übersetzer und Renderer der Spielwelt, nicht die Quelle der Wahrheit für Cluster-Zustand — der echte Cluster ist die Quelle der Wahrheit.

---

## 2. Komponentenüberblick

```
┌─────────────────┐     REST + WebSocket    ┌──────────────────┐     client-go     ┌─────────────────┐
│  Frontend        │ ◄─────────────────────► │  Backend           │ ◄────────────────► │  kind-Cluster    │
│  (Phaser + TS)   │                          │  (Go)              │                     │  (echte Nodes,   │
│  läuft im Browser│                          │  - Aktions-API     │                     │   Pods, Deploy-  │
│                  │                          │  - State-Watcher   │                     │   ments)         │
└─────────────────┘                          │  - Chaos-Service   │                     └─────────────────┘
                                              │  - Fortschritt     │
                                              │    (lokal, SQLite) │
                                              └──────────────────┘
```

- **Frontend** — Phaser (2D Canvas/WebGL) + TypeScript. Rendert die Fabrik, nimmt Spieler-Absichten entgegen, hält keinen eigenen Spielzustand (nur Rendering-State). Wird vom Backend als statische Dateien via Go `embed` ausgeliefert.
- **Backend** — Go, `client-go`. Einzige API-Schicht; das Frontend spricht nie direkt mit dem Cluster. Übersetzt Spieler-Absichten in K8s-Objekte, watcht echten Cluster-Zustand, übersetzt Events zurück in Fabrik-Vokabular, verwaltet lokalen Fortschritt.
- **Chaos-Service** — eigene Goroutine im Backend. Killt ab Level 4 nach level-abhängigen, konfigurierbaren Regeln (Intervall/Wahrscheinlichkeit, kein Hardcoding) echte Pods, um Maschinenausfälle zu erzeugen. Meldet nichts selbst ans Frontend — das läuft automatisch über den normalen Watch-Mechanismus.
- **kind-Cluster** — echter lokaler Kubernetes-Cluster, erzeugt vom Start-Skript.

---

## 3. Level-Gating

Level-Fortschritt bestimmt, welchen K8s-Sprachumfang das Backend verwendet — keine künstliche Sperre, sondern schrittweise erweiterter Übersetzungscode:

| Level | Konzept | Backend-Verhalten |
|---|---|---|
| 1 | Container | Erzeugt nackte Einzel-Pods, ein Container |
| 2 | Pod | Pods mit mehreren Containern (Sidecar) |
| 3 | Halle & Einplanung | Nodes mit realen Resource Requests/Limits; Scheduler-Entscheidungen werden im Frontend sichtbar |
| 4 | Auftrag & Selbstheilung | Wechsel von Pod- auf Deployment-Erzeugung; Chaos-Service aktiviert sich erstmals |
| 5 | Werk erweitern | Weitere Nodes im kind-Cluster |
| 6 | Verladerampe | Service-Objekte |
| 7 | Bauplan | Echtes Git-Repo + Reconciliation-Loop (GitOps) |
| 8 | Umbau im laufenden Betrieb | Rolling Updates |

Der Spieler tippt nie YAML oder kubectl-Befehle direkt. Die Fabrik-UI erzeugt Absichten (z. B. "100 Teile/Minute"), das Backend generiert daraus die passenden K8s-Objekte für den freigeschalteten Sprachumfang. Das hält das didaktische Prinzip "Erleben vor Benennen" ein — Fachbegriffe erscheinen erst nach der Aktion (Tooltip/Popup), nie vorher.

**Ab Level 4** gibt es ein optionales "Unter der Haube"-Fenster, das das echte generierte YAML bzw. kubectl-Output der zugrundeliegenden K8s-Objekte zeigt.

---

## 4. Datenfluss

1. Spieler trifft eine Absicht in der Fabrik-UI (z. B. "Auftrag: 100 Teile/Minute für Bauteil X").
2. Frontend schickt die Aktion per REST ans Backend.
3. Backend übersetzt sie in K8s-Objekte passend zum freigeschalteten Level-Vokabular und wendet sie über `client-go` auf den kind-Cluster an.
4. Backend hält `client-go`-Informer/Watches auf relevante Objekttypen (Pods, Nodes, Deployments). Jede reale Zustandsänderung (Scheduling, Neustart, Ausfall) kommt als Event rein.
5. Backend übersetzt Events zurück in Fabrik-Vokabular und pusht sie per WebSocket ans Frontend.
6. Frontend aktualisiert die Werk-Darstellung rein reaktiv.
7. Fortschritt (aktuelles Level, Meilensteine) wird bei Level-Abschluss in SQLite geschrieben.

Der Chaos-Service läuft parallel und braucht keinen eigenen Meldeweg — gelöschte Pods laufen automatisch durch Schritt 4–6.

---

## 5. Setup & Start

- **`make start`** (bzw. `./scripts/start.sh`): prüft Docker-Installation/-Status mit laienverständlicher Fehlermeldung, erstellt den kind-Cluster (`kind create cluster --name podustrial --config kind-config.yaml`) mit Node-Anzahl/-Kapazität passend zum aktuellen Fortschritt, startet das Backend-Binary (Frontend eingebettet via Go `embed`) — ein Prozess, ein Port.
- **`make stop`**: löscht den kind-Cluster sauber (`kind delete cluster`).
- **`make dev`**: Entwickler-Modus mit Hot-Reload (separater Vite-Dev-Server, proxied zum Go-Backend) — nicht für den Spieler gedacht.

---

## 6. Fehlerbehandlung & Edge Cases

- **Docker fehlt/läuft nicht:** Start-Skript prüft vorab, klare Fehlermeldung ohne K8s-Jargon, Link zur Installation.
- **kind-Cluster-Erstellung schlägt fehl:** Skript räumt einen halb erstellten Cluster automatisch weg (`kind delete cluster`), bevor es abbricht — kein kaputter Restzustand.
- **Backend verliert Cluster-Verbindung** (z. B. Docker Desktop-Neustart): über Informer-Fehlerstatus erkannt, Frontend zeigt einen fabrik-passenden Zustand ("Stromausfall im Werk") statt technischem Fehler, automatischer Reconnect-Versuch.
- **Chaos-Service zu aggressiv:** Ausfallraten pro Level liegen in Config-Werten, nicht im Code — balancierbar ohne Codeänderung.
- **Spieler schließt Browser/Prozess mitten in einer Aktion:** unkritisch, da der reale Cluster-Zustand die Quelle der Wahrheit ist. Beim nächsten Start liest das Backend den realen Zustand neu ein.

---

## 7. Testing-Strategie

- **Backend-Übersetzungslogik** (Absicht → K8s-Objekt, Event → Fabrik-Vokabular): reine Unit-Tests ohne echte K8s-Abhängigkeit.
- **Backend-Integration:** Tests gegen `envtest` oder einen echten kind-Test-Cluster in CI — prüfen, dass ein "Auftrag"-Request zum erwarteten Deployment führt und simulierter Ausfall zu echtem Self-Healing.
- **Chaos-Service:** deterministisch testbar über injizierbaren Zufallsgenerator/Clock.
- **Frontend:** Komponenten-/Rendering-Tests isoliert vom Backend, über gemockte WebSocket-Events.
- **End-to-End:** von Anfang an automatisiert mit Playwright gegen den echten lokalen Stack (Backend + kind).

---

## 8. Scope-Grenzen dieser Spec

**Abgedeckt:** technische Grundarchitektur, Level-Gating-Mechanismus, Datenfluss, Setup/Start, Fehlerbehandlung, Testing-Strategie.

**Explizit nicht Teil dieser Spec** — offene Punkte für spätere Sessions:
- Konkrete Spielmechanik und Gewinnbedingungen pro Level
- Wie weit die Fabrik-Metapher trägt, bevor auf echtes K8s-Vokabular umgeschaltet wird
- Wie viel "Bauen" der Spieler selbst tun darf, ohne das deklarative Prinzip zu verwässern
- Arbeitsaufteilung zwischen den beiden Beteiligten
- Der spätere Modus mit geteiltem Cluster für mehrere Spieler — die Architektur ist so geschnitten, dass der Backend-Code dafür wiederverwendbar bleibt, das Wie ist aber nicht spezifiziert
