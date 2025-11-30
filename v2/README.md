# GoSQLPage v2.1 — Architecture Consolidée

Un serveur web SQL-driven où :
- Les pages sont des fichiers SQL
- Les blocs sont l'unité fondamentale (inspiré SiYuan, simplifié)
- Les mutations passent par des sessions isolées
- Le merge est le seul point d'écriture sur le contenu permanent
- Tout est auditable, rejouable, résilient

## Principes

1. **Read = direct, Write = isolé** — Lecture sur content.db, écriture dans session.db éphémère
2. **Un fichier = un contexte** — Pas d'ATTACH, pas de transactions cross-db
3. **Le merger est gardien** — Singleton avec recovery, seul writer sur content.db, transactionnel
4. **Les blocs portent du sens** — Pas juste du formatage, des types sémantiques
5. **Le schéma est la documentation** — SQL lisible = système compréhensible
6. **Fail safe, recover fast** — Crash d'un composant n'affecte pas les autres

## Structure des fichiers

```
v2/
├── cmd/
│   └── gopage-v2/          # Point d'entrée v2.1
│       └── main.go
├── config/                  # Configuration
│   ├── gosqlpage.toml
│   ├── merger.toml
│   ├── gc.toml
│   └── audit.toml
├── data/                    # Schémas SQL
│   ├── schema_content.sql   # Blocs, relations, vecteurs
│   ├── schema_schema.sql    # Types de blocs, validation
│   ├── schema_users.sql     # Auth, permissions
│   ├── schema_audit.sql     # Logs, traçabilité
│   └── schema_session.sql   # Session de travail
├── pkg/
│   ├── api/                 # HTTP API handlers
│   ├── audit/               # Audit logging
│   ├── blocks/              # Types de blocs, ID, fractional indexing
│   ├── gc/                  # Garbage collection
│   ├── merger/              # Daemon de merge
│   ├── session/             # Gestion des sessions
│   └── users/               # Types utilisateur/permissions
├── sessions/                # Sessions actives (runtime)
├── queue/                   # Queue de merge
│   ├── pending/
│   ├── processing/
│   ├── done/
│   └── failed/
├── cache/
│   ├── pages/
│   └── queries/
└── backup/
```

## Utilisation

### Initialiser les bases de données

```bash
go run ./v2/cmd/gopage-v2 -init
```

### Démarrer le serveur

```bash
go run ./v2/cmd/gopage-v2 -port 8080 -debug
```

### API Endpoints

#### Sessions

```bash
# Créer une session
curl -X POST http://localhost:8080/api/session

# Ajouter un bloc à la session
curl -X POST http://localhost:8080/api/session/blocks \
  -H "Content-Type: application/json" \
  -d '{"block": {"type": "paragraph", "content": "Hello World"}}'

# Soumettre pour merge
curl -X POST http://localhost:8080/api/session/submit
```

#### Blocs

```bash
# Lister les blocs
curl http://localhost:8080/api/blocks

# Obtenir un bloc
curl http://localhost:8080/api/blocks/{id}

# Rechercher
curl http://localhost:8080/api/search?q=hello
```

#### Admin

```bash
# Schéma des types de blocs
curl http://localhost:8080/api/admin/schema

# État des queues
curl http://localhost:8080/api/admin/queue

# Audit logs
curl http://localhost:8080/api/admin/audit
```

## Types de blocs

### Catégorie: Contenu
- `document` - Racine d'un document
- `heading` - Titre de section
- `paragraph` - Texte courant
- `list` / `list_item` - Listes
- `code` - Bloc de code
- `table` - Tableau
- `quote` - Citation
- `embed` - Contenu embarqué

### Catégorie: Discussion
- `question` - Question posée
- `answer` - Réponse
- `claim` - Affirmation
- `counter` - Contre-argument
- `evidence` - Preuve/source

### Catégorie: Connaissance
- `definition` - Définition d'un terme
- `procedure` - Étapes à suivre
- `decision` - Décision prise
- `lesson` - Apprentissage extrait

### Catégorie: Tâches
- `task` - Tâche à faire
- `milestone` - Jalon
- `blocker` - Obstacle

### Catégorie: Bot/LLM
- `bot_request` - Demande au bot
- `bot_response` - Réponse du bot
- `bot_reasoning` - Trace de raisonnement

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       HTTP Request                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      GoSQLPage Server                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   API       │  │   SQL       │  │      Renderer       │ │
│  │  Handler    │  │   Engine    │  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
        │                                       │
        ▼                                       ▼
┌───────────────┐                    ┌───────────────────────┐
│ Session       │                    │     content.db        │
│ Manager       │                    │     (read-only)       │
│               │                    └───────────────────────┘
│ ┌───────────┐ │
│ │session.db │ │ ──▶ queue/pending/ ──▶ ┌───────────────────┐
│ │session.db │ │                        │     Merger        │
│ │session.db │ │                        │    (singleton)    │
│ └───────────┘ │                        │                   │
└───────────────┘                        │    content.db     │
                                         │    (write)        │
                                         └───────────────────┘
                                                  │
                                                  ▼
                                         ┌───────────────────┐
                                         │    audit.db       │
                                         │    (logging)      │
                                         └───────────────────┘
```

## Garanties

| Propriété | Mécanisme |
|-----------|-----------|
| Isolation | Session.db par utilisateur, aucun ATTACH |
| Atomicité | Merge transactionnel, tout ou rien |
| Traçabilité | Audit.db log chaque mutation |
| Résilience | Crash session = seulement cette session perdue |
| Recovery | Merger reprend queue/processing/ au startup |
| Concurrence | N sessions parallèles, merge séquencé |
| Conflits | Détection hash + structure, résolution explicite |
| Permissions | Granulaires, vérifiées à chaque étape |
| Cache | Invalidation ciblée au merge |
| Extensibilité | Types configurables dans schema.db |

## Roadmap

### Phase 1: Fondations ✓
- [x] Schéma content.db (blocks, refs, attrs)
- [x] Schéma schema.db (block_types minimal)
- [x] Schéma session.db
- [x] Session manager (create, open, close)
- [x] Merge simple (sans conflits)

### Phase 2: Robustesse
- [x] Détection de conflits (hash)
- [x] Queue et merger daemon
- [x] Audit.db logging
- [x] Permissions basiques
- [x] Healthchecks
- [x] GC

### Phase 3: Fonctionnalités
- [x] Types de blocs sémantiques
- [x] Relations typées
- [x] Recherche FTS
- [ ] Interface résolution conflits
- [x] API complète
- [ ] Cache pages

### Phase 4: Avancé
- [ ] Multi-tenant
- [ ] Intégration bot/LLM
- [ ] Vecteurs pour RAG
- [ ] Backup automatisé
- [ ] Métriques Prometheus
- [ ] Documentation auto-générée
