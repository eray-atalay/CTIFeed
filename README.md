# CTIFeed - Cyber Threat Intelligence (CTI) Aggregator & ThreatRadar

**CTIFeed**, siber tehdit istihbaratı (Cyber Threat Intelligence) kaynaklarından RSS/Atom beslemelerini ve Telegram kanallarını eşzamanlı toplayan, içerikleri kural tabanlı dinamik puanlama motoruyla önceliklendiren, haberlerden otomatik Tehdit Göstergeleri (IoC) çıkaran ve modern Astro web arayüzü sunan Go tabanlı bir CTI platformudur.

---

## Özellikler

- **Modern Web Dashboard:** Astro ve TypeScript ile geliştirilmiş, gömülü (`go:embed`) varlıklarla tek bir binary olarak çalışan koyu temalı arayüz.
- **Eşzamanlı Veri Toplama (Worker Pool):** Ayarlanabilir worker havuzu (goroutines) ve her istek için `context.WithTimeout` koruması.
- **25 Güvenilir CTI Kaynağı:** The Hacker News, BleepingComputer, SecurityWeek, Unit 42, Cisco Talos, The DFIR Report, Securelist, Dark Reading, Microsoft Threat Intelligence, Krebs on Security, ANY.RUN, CrowdStrike ve Telegram kanalları (cveNotify, breachdetect).
- **Dinamik Önceliklendirme Motoru (Cumulative Scoring):**
  - **TR-Focus (+50 Puan):** Türkiye kurumları, kritik altyapı ve regülatör (USOM, BTK, BDDK vb.) referansları.
  - **Regex Tabanlı CVE Tespiti (+35 Puan):** `(?i)\bCVE-\d{4}-\d{4,7}\b` kalıbıyla otomatik ayıklama ve etiketleme.
  - **Kritik Ürün ve Servisler (+30 Puan):** Fortinet, Palo Alto, Cisco, Ivanti, VMware, Exchange, Active Directory vb.
  - **Aktif Sömürü ve PoC (+25 Puan):** In-the-wild, CISA KEV ve PoC tespitleri.
  - **Kritik Tehdit Vektörleri (+20 Puan):** Zero-day, RCE, auth-bypass, ransomware, data breach vb.
- **Otomatik IoC Çıkarımı ve Dışa Aktarma:**
  - IPv4, Alan Adı (Domain), SHA256 ve MD5 hash tespiti.
  - Yanlış pozitifleri önlemek için defang edilmiş format (`evil[.]com`, `hxxps://`) ayrıştırma ve beyaz liste doğrulaması.
  - Firewall/EDR entegrasyonları için TXT veya CSV blok listesi olarak dışa aktarım (`/api/iocs/export`).
- **Telegram Bot Entegrasyonu:**
  - Gerçek zamanlı kritik tehdit bildirimleri (`/start`, `/kapsam`, `/filtre`).
  - Kategori ve zaman aralığına göre dinamik filtre menüsü.
- **Analitik Grafikler ve Trendler:** Kaynak dağılımı, haftalık saldırı zaman çizelgesi ve hedeflenen kurumsal teknolojiler.
- **Pure Go SQLite (CGO-Free):** Harici C derleyicisine ihtiyaç duymadan doğrudan derlenebilen `modernc.org/sqlite` altyapısı.
- **Graceful Shutdown:** SIGINT/SIGTERM sinyallerinde veritabanı ve arka plan işlerini temiz sonlandırma.

---

## Proje Mimarisi

```
CTIFeed/
├── cmd/
│   └── ctifeed/              # CLI / Web sunucu giriş noktası
├── frontend/                 # Astro tabanlı modern web arayüzü
│   ├── src/                  # Bileşenler, sayfalar, stiller
│   └── astro.config.mjs
├── internal/
│   ├── api/                  # REST API ve statik dosya sunucusu
│   ├── collector/            # RSS/Atom ve Telegram toplayıcı
│   ├── config/               # Besleme kaynakları ve yapılandırma
│   ├── ioc/                  # Regex ve defang tabanlı IoC çıkarıcı
│   ├── model/                # Veri modelleri (Article, IoC)
│   ├── notifier/             # Telegram bot ve bildirim mekanizması
│   ├── scorer/               # Önceliklendirme ve puanlama motoru
│   └── storage/              # SQLite depolama ve analitik sorguları
├── web/                      # Gömülü statik varlıklar (embed.FS)
├── Dockerfile                # Multi-stage Docker imajı
├── docker-compose.yml        # Compose servis tanımı
├── .env.example              # Örnek ortam değişkenleri
├── go.mod
├── go.sum
├── LICENSE                   # MIT Lisansı
└── README.md
```

---

## Kurulum ve Derleme

### Gereksinimler
- Go 1.22+
- Node.js 18+ (Astro arayüzünü derlemek için)

### Derleme Adımları

```bash
# 1. Frontend varlıklarını derleyin:
cd frontend
npm install
npm run build
cd ..

# 2. Go binary dosyasını derleyin:
go build -o bin/ctifeed ./cmd/ctifeed
```

---

## Kullanım

### 1. Web Dashboard (Varsayılan)

```bash
./bin/ctifeed
```

Tarayıcınızda açın:
`http://localhost:8080`

Farklı bir port kullanmak için:
```bash
./bin/ctifeed -port 3000
```

### 2. Docker ile Çalıştırma

```bash
docker compose up -d
```

Uygulama `http://localhost:8081` adresinde çalışır ve SQLite veritabanı `./data` dizininde kalıcı olarak saklanır.

### 3. CLI Terminal Modu

Web arayüzü yerine terminal üzerinde özet rapor almak için `-cli` bayrağı kullanılabilir:

```bash
./bin/ctifeed -cli -top 10 -min-score 50
```

---

## REST API Uç Noktaları

| Metot | Uç Nokta | Açıklama |
|---|---|---|
| `GET` | `/api/stats` | Toplam makale, yüksek öncelikli, CVE ve TR-Focus sayıları |
| `GET` | `/api/analytics` | Kaynak payları, aktivite zaman çizelgesi, popüler etiketler |
| `GET` | `/api/articles` | Filtrelenmiş ve sayfalanmış haberler (`search`, `tag`, `source`, `min_score`, `time_range`, `limit`, `offset`) |
| `GET` | `/api/iocs` | Tespit edilen IoC listesi (`type`, `search`, `article_id`, `limit`, `offset`) |
| `GET` | `/api/iocs/export` | IoC blok listesi indirme (`type=ip\|domain\|sha256`, `format=txt\|csv`) |
| `GET` | `/api/sources` | Taranan besleme kaynaklarının listesi |
| `POST`| `/api/scan` | Anlık tarama döngüsünü tetikler |

---

## Yapılandırma Seçenekleri

| Bayrak | Ortam Değişkeni | Varsayılan | Açıklama |
|---|---|---|---|
| `-port` | `PORT` | `8080` | Web sunucu portu |
| `-cli` | - | `false` | CLI konsol modunu etkinleştirir |
| `-interval` | `INTERVAL` | `15m` | Otomatik tarama periyodu |
| `-workers` | `WORKERS` | `5` | Eşzamanlı worker sayısı |
| `-timeout` | `TIMEOUT` | `10s` | İstek zaman aşımı süresi |
| `-db` | `DB_PATH` | `ctifeed.db` | SQLite veritabanı dosya yolu |
| `-max-age` | `MAX_AGE` | `168h` | İşlenecek makalelerin azami yaşı |
| `-telegram-token` | `TELEGRAM_BOT_TOKEN` | `""` | Telegram Bot API Token |
| `-verbose` | - | `false` | Detaylı debug logları |

---

## Lisans

Bu proje [MIT](LICENSE) lisansı altında sunulmaktadır.
