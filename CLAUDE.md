# CLAUDE.md

Leitfaden für Agents (und Mitwirkende), die eines der GitHub-Issues in diesem Repo abarbeiten.

## Projektüberblick

Podustrial ist ein browserbasiertes Lernspiel, das Kubernetes-Grundkonzepte über eine Fabrik-Metapher vermittelt (Zielgruppe: Menschen ohne Container-/K8s-Vorwissen). Die kanonischen Quellen — nicht in dieser Datei duplizieren, sondern nachschlagen:

- **Grundidee & Didaktik:** `podustrial-konzept.md`
- **Technische Architektur (Spec):** `docs/superpowers/specs/2026-09-01-technische-architektur-design.md`
- **Implementierungsplan mit vollständigen TDD-Schritten inkl. Code:** `docs/superpowers/plans/2026-09-01-technische-architektur.md`

Die aktuelle Arbeit ist in GitHub Issues aufgeteilt: 7 Epic-Issues (grobe Bereiche) mit je 1–3 Task-Sub-Issues (konkrete, einzeln testbare Deliverables, entsprechen 1:1 den Tasks im Plan-Dokument).

## Workflow pro Issue

1. **Epic wählen.** Prüfe, ob ein Epic-Issue bereits einen Assignee hat: `gh issue view <epic-nummer> --json assignees`. Ist es zugewiesen, wähle ein anderes Epic. **Nur ein Agent bearbeitet ein Epic gleichzeitig** — das reserviert alle darin enthaltenen Tasks exklusiv, um Merge-Konflikte durch parallele Arbeit im selben Bereich zu vermeiden.
2. **Epic reservieren.** Weise dich dem Epic-Issue zu: `gh issue edit <epic-nummer> --add-assignee @me`.
3. **Task wählen und reservieren.** Wähle innerhalb des Epics ein offenes Task-Issue, dessen Abhängigkeiten (siehe Abschnitt "Abhängigkeit" im Task-Issue-Text) bereits gemerged sind. Weise dich auch diesem Issue zu: `gh issue edit <task-nummer> --add-assignee @me`.
4. **Task-Details nachschlagen.** Das Task-Issue verweist auf die Task-Nummer im Plan-Dokument (`docs/superpowers/plans/2026-09-01-technische-architektur.md`). Dort stehen die vollständigen TDD-Schritte inkl. Code — das Issue selbst enthält nur eine Zusammenfassung.
5. **Branch erstellen.** Namensschema: `task-<issue-nummer>-<kurzslug>`, z. B. `task-8-setup-scripts`.
6. **Strikt TDD arbeiten**, Schritt für Schritt wie im Plan beschrieben: fehlschlagenden Test schreiben → Fehlschlag verifizieren → minimale Implementierung → Erfolg verifizieren → committen. Commit-Präfixe: `feat:`, `test:`, `fix:`, `chore:`, `ci:`.
7. **Lokal selbst gegenprüfen, bevor der Task als fertig gilt.** Automatisierte Tests grün sind eine Voraussetzung, aber kein Ersatz für den echten Funktionstest: `make start` lokal ausführen und die im Task umgesetzte Funktionalität tatsächlich manuell prüfen (z. B. per `curl` gegen die neue Route, im Browser die gerenderte Szene ansehen, `kubectl get pods` gegen den echten kind-Cluster). Erst danach den PR öffnen.
8. **PR öffnen.** Gegen `main`, PR-Beschreibung mit `Closes #<task-nummer>`. Kein Auto-Merge — Review durch einen Menschen ist erforderlich, bevor gemergt wird.
9. **Epic abschließen.** Sind alle Task-Issues eines Epics gemergt, schließe das Epic-Issue manuell (`gh issue close <epic-nummer>`) — GitHub schließt Parent-Issues bei Sub-Issues nicht automatisch.
10. **Vorzeitig abbrechen.** Wird die Arbeit an einem Epic nicht fortgesetzt, bevor alle seine Tasks fertig sind, entferne dich als Assignee (`gh issue edit <epic-nummer> --remove-assignee @me`), damit das Epic für andere wieder frei wird.

## Befehle

- `make start` — lokalen kind-Cluster erstellen (falls nötig) und den Server starten (Spieler-Modus, ein Prozess/Port, eingebettetes Frontend)
- `make stop` — kind-Cluster wieder löschen
- `make dev` — Entwickler-Modus mit Hot-Reload (Frontend-Dev-Server + Go-Backend), nicht für den Spieler gedacht
- `make test` — alle Unit-Tests (Go + Frontend)
- `make test-integration` — Integrationstests gegen einen frischen, temporären kind-Test-Cluster
- `go test ./internal/<paket>/... -v` — gezielt ein einzelnes Go-Paket testen
- `cd frontend && npx vitest run <pfad>` — gezielt eine einzelne Frontend-Testdatei ausführen

## Globale Leitplanken (aus der Spec)

- **Echte Kubernetes-Objekte, keine Simulation.** Spieleraktionen erzeugen echte Pods/Deployments im echten kind-Cluster; Scheduling und Self-Healing kommen vom echten K8s-Scheduler/Controller.
- **Chaos-Raten sind Config-Werte**, nie im Code hartkodiert.
- **Scope ist lokal, Single-Player.** Kein Mehrspieler- oder geteilter-Cluster-Code in diesem Plan — das ist explizit eine spätere Erweiterung.
- **Das Backend ist die einzige Schicht, die den Cluster anfasst.** Das Frontend spricht nie direkt mit Kubernetes, nur über die Backend-API/WebSocket.
- **Kein Overengineering über den jeweiligen Task hinaus.** Jeder Task hat einen klar definierten Umfang inkl. exakter Interfaces, die spätere Tasks konsumieren — an diesen Signaturen nichts ändern, ohne die abhängigen Tasks/Issues zu prüfen.

## Bekannte Abhängigkeiten zwischen Tasks

- Task 8 (WebSocket Hub) ändert die Signatur von `api.NewServer` aus Task 7 (`NewServer(k8sClient)` → `NewServer(k8sClient, hub)`). Vor Arbeit an Task 8 prüfen, dass Task 7 bereits gemerged ist.
- Task 10 (Chaos Service) ist interface-kompatibel mit Task 4s `ListManagedPodNames`/`DeletePod`, aber im eigenen Unit-Test vollständig gemockt — kann parallel zu Task 4 entstehen, sofern das jeweils andere Epic nicht gerade exklusiv reserviert ist.
- Task 11 (Server Wiring) benötigt alle Backend-Epics (Setup, Kubernetes Translation Layer, Backend API & Realtime, Progress & Chaos Service) fertig gemerged.
- Task 14 (E2E-Test) benötigt Task 11 (Server-Binary) und Task 13 (Frontend rendert ein `<canvas>`).
