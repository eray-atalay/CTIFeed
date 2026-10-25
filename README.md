<p align="center">
  <img src="img/logo.png" alt="CTIFeed Logo" width="130" height="130" />
</p>

# CTIFeed - Cyber Threat Intelligence (CTI) Aggregator & ThreatRadar

**CTIFeed**, siber tehdit istihbaratı (Cyber Threat Intelligence) kaynaklarından RSS/Atom beslemelerini, Twitter/X köprüsünü ve Telegram kanallarını eşzamanlı toplayan, toplanan verileri kural tabanlı dinamik puanlama motoruyla önceliklendiren, içeriklerden otomatik Tehdit Göstergeleri (IoC) çıkaran, MySQL/GORM altyapısına sahip ve modern Astro/Preact web arayüzü sunan Go tabanlı kurumsal bir CTI platformudur.

---

## Özellikler

- **Modern Web Dashboard:** Astro, Preact ve TypeScript ile geliştirilmiş, gömülü (`go:embed`) üretim varlıklarıyla tek bir binary olarak çalışan koyu temalı (SOC odaklı) arayüz.
- **Özel Tehdit Radarları:**
  - **CVE Zafiyet Radarı:** Güncel CVE kodları, CVSS etiketleri ve aktif istismar risklerini takip eden dinamik görünüm.
  - **Veri Sızıntısı (Breach) Radarı:** Telegram ve sızıntı istihbarat kanallarından derlenen veri ihlali bildirimleri.
- **Eşzamanlı Veri Toplama (Worker Pool):** Ayarlanabilir worker havuzu (goroutines) ve her istek için `context.WithTimeout` koruması.
- **Geniş CTI Kaynak Yelpazesi:** The Hacker News, BleepingComputer, SecurityWeek, Unit 42, Cisco Talos, The DFIR Report, Securelist, Dark Reading, Microsoft Threat Intelligence, Krebs on Security, ANY.RUN, CrowdStrike, Twitter/X akışları ve Telegram istihbarat kanalları (cveNotify, breachdetect).
- **Dinamik Önceliklendirme Motoru (Cumulative Scoring):**
  - **TR-Focus (+50 Puan):** Türkiye kurumları, kritik altyapı, telekom, finans ve regülatör (USOM, BTK, BDDK vb.) referansları.
  - **Regex Tabanlı CVE Tespiti (+35 Puan):** `(?i)\bCVE-\d{4}-\d{4,7}\b` kalıbıyla otomatik ayıklama ve etiketleme.
  - **Kritik Ürün ve Servisler (+30 Puan):** Fortinet, Palo Alto, Cisco, Ivanti, VMware, Exchange, Active Directory vb.
  - **Aktif Sömürü ve PoC (+25 Puan):** In-the-wild, CISA KEV ve PoC tespitleri.
  - **Kritik Tehdit Vektörleri (+20 Puan):** Zero-day, RCE, auth-bypass, ransomware, veri sızıntısı vb.
- **Otomatik IoC Çıkarımı ve Dışa Aktarma:**
  - IPv4, Alan Adı (Domain), SHA256 ve MD5 hash tespiti.
  - Yanlış pozitifleri (false-positive) önlemek için defang edilmiş format (`evil[.]com`, `hxxps://`) ayrıştırma ve beyaz liste doğrulaması.
  - Firewall/SIEM/EDR entegrasyonları için TXT veya CSV formatında blok listesi indirme (`/api/iocs/export`).
- **Telegram Bot Entegrasyonu:**
  - Gerçek zamanlı kritik tehdit bildirimleri (`/start`, `/abone`, `/son`).
  - Etiket ve kritiklik seviyesine göre interaktif butonlu abonelik menüsü.
- **Yönetici (Admin) Paneli ve Kaynak Yönetimi:**
  - JWT tabanlı güvenli oturum yönetimi.
  - CTI besleme kaynaklarının anlık sağlık/gecikme durumları, kaynak ekleme/düzenleme ve açma/kapatma (toggle) kontrolleri.
- **MySQL & GORM Veritabanı Mimarisi:** İlişkisel veri modeli, indekslenmiş IoC ve makale sorguları.
- **Graceful Shutdown:** SIGINT/SIGTERM sinyallerinde veritabanı bağlantılarını ve arka plan işlerini temiz sonlandırma.

---

## Proje Mimarisi

```
CTIFeed/
├── cmd/
│   └── ctifeed/              # CLI / Web sunucu giriş noktası (main.go)
├── frontend/                 # Astro & Preact tabanlı modern web arayüzü
│   ├── src/                  # Sayfalar, bileşenler, ada (island) mimarisi
│   └── astro.config.mjs
├── internal/
│   ├── api/                  # REST API, JWT kimlik doğrulama ve statik dosya sunucusu
│   ├── collector/            # RSS/Atom, Twitter ve Telegram toplayıcı motoru
│   ├── config/               # Besleme kaynakları ve konfigürasyon yönetimi
│   ├── ioc/                  # Regex ve defang tabanlı IoC çıkarıcı
│   ├── model/                # GORM veri modelleri (Article, IoC, FeedSource)
│   ├── notifier/             # Telegram botu ve bildirim dağıtıcısı
│   ├── scorer/               # Puanlama motoru ve TR-Focus tespit kuralları
│   └── storage/              # MySQL veritabanı katmanı ve analitik sorgular
├── web/                      # Gömülü statik üretim varlıkları (embed.FS)
├── Dockerfile                # Çok aşamalı (multi-stage) Docker imajı
├── docker-compose.yml        # MySQL ve CTIFeed servis tanımları
├── .env.example              # Örnek ortam değişkenleri şablonu
├── go.mod
├── go.sum
├── LICENSE                   # MIT Lisansı
└── README.md
```

---

## Kurulum ve Yapılandırma

### Gereksinimler
- **Go:** 1.22+
- **Node.js:** 18+ (Astro arayüzünü derlemek için)
- **MySQL:** 8.0+ (veya Docker Compose)

### 1. Ortam Değişkenlerinin Hazırlanması

Depodaki örnek şablonu kopyalayarak kendi `.env` dosyanızı oluşturun:

```bash
cp .env.example .env
```

`.env` dosyasını bir metin düzenleyiciyle açıp ortamınıza uygun değerleri (parolalar, portlar ve isteğe bağlı Telegram/Twitter token'ları) tanımlayın.

> **Güvenlik Notu:** `.env` dosyanız hassas bilgiler içerebileceği için kesinlikle Git'e commit edilmemelidir (`.gitignore` dosyasında varsayılan olarak engellenmiştir).

---

### 2. Derleme Adımları

```bash
# 1. Frontend arayüzünü derleyin:
cd frontend
npm install
npm run build
cd ..

# 2. Go binary dosyasını derleyin:
go build -o bin/ctifeed ./cmd/ctifeed
```

---

## Kullanım

### 1. Docker Compose ile Çalıştırma (Önerilen)

Tüm sistemi (MySQL ve CTIFeed) tek bir komutla ayağa kaldırabilirsiniz:

```bash
docker compose up -d
```

- **Web Dashboard:** `http://localhost:8088`
- **MySQL Portu:** `3307` (Dış erişim için)

Logları izlemek için:
```bash
docker compose logs -f ctifeed
```

Durdurmak için:
```bash
docker compose down
```

### 2. Yerel Olarak Çalıştırma

```bash
./bin/ctifeed
```

Farklı bir port kullanmak için:
```bash
./bin/ctifeed -port 3000
```

### 3. CLI Terminal Modu

Web arayüzü yerine terminalde hızlı özet rapor almak için `-cli` parametresini kullanabilirsiniz:

```bash
./bin/ctifeed -cli -top 10 -min-score 50
```

---

## REST API Uç Noktaları

### Genel Uç Noktalar

| Metot | Uç Nokta | Açıklama |
|---|---|---|
| `GET` | `/api/stats` | Toplam makale, yüksek öncelikli, CVE ve TR-Focus istatistikleri |
| `GET` | `/api/analytics` | Kaynak dağılımı, zaman çizelgesi ve popüler etiketler |
| `GET` | `/api/articles` | Filtrelenmiş ve sayfalanmış haberler (`search`, `tag`, `source`, `min_score`, `time_range`, `limit`, `offset`) |
| `GET` | `/api/iocs` | Tespit edilen IoC kayıtları (`type`, `search`, `article_id`, `limit`, `offset`) |
| `GET` | `/api/iocs/export` | IoC blok listesi indirme (`type=ip\|domain\|sha256\|md5`, `format=txt\|csv`) |
| `GET` | `/api/sources` | Taranan CTI kaynakları ve sağlık durumları |
| `POST`| `/api/scan` | Anlık CTI tarama döngüsünü tetikler |

### Yönetici (Admin) Uç Noktaları

| Metot | Uç Nokta | Açıklama |
|---|---|---|
| `POST`| `/api/admin/login` | Yönetici girişi ve JWT oturum çerezi alma |
| `POST`| `/api/admin/logout`| Yönetici oturumunu sonlandırma |
| `POST`| `/api/admin/sources/toggle` | Belirli bir besleme kaynağını aktif/pasif yapma |
| `POST`| `/api/admin/sources` | Yeni CTI besleme kaynağı ekleme |

---

## Yapılandırma Seçenekleri (Environment Variables & Flags)

| Bayrak | Ortam Değişkeni | Varsayılan | Açıklama |
|---|---|---|---|
| `-port` | `PORT` | `8080` | Web sunucu portu |
| `-cli` | - | `false` | CLI konsol modunu etkinleştirir |
| `-interval` | `INTERVAL` | `15m` | Besleme tarama periyodu |
| `-workers` | `WORKERS` | `5` | Eşzamanlı worker sayısı |
| `-timeout` | `TIMEOUT` | `10s` | İstek zaman aşımı süresi |
| `-dsn` | `MYSQL_DSN` | `""` | MySQL DSN bağlantı dizesi |
| `-db-host` | `DB_HOST` | `127.0.0.1` | MySQL sunucu adresi |
| `-db-port` | `DB_PORT` | `3306` | MySQL sunucu portu |
| `-db-user` | `DB_USER` | `ctifeed` | MySQL kullanıcı adı |
| `-db-password` | `DB_PASSWORD` | - | MySQL şifresi (Ortam değişkeninden alınır) |
| `-db-name` | `DB_NAME` | `ctifeed` | MySQL veritabanı adı |
| `-max-age` | `MAX_AGE` | `168h` | İşlenecek makalelerin azami yaşı |
---

## Lisans

Bu proje [MIT](LICENSE) lisansı altında sunulmaktadır.
