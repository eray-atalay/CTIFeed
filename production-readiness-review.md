# CTIFeed Production Readiness Review

Bu doküman, CTIFeed projesinin canlı ortama alınması öncesinde tespit edilen güvenlik, operasyon, veri kaybı ve performans risklerini içerir.

## Genel Karar

Proje geliştirme ve test ortamında çalışır durumdadır. Ancak mevcut haliyle doğrudan internete açılması önerilmez.

Canlıya geçmeden önce özellikle şu üç konu çözülmelidir:

1. Varsayılan admin kimlik bilgilerinin devre dışı bırakılması
2. HTTPS ve güvenli cookie kullanımı
3. MySQL backup ve restore planının oluşturulması

## Kritik Riskler

### 1. Varsayılan Admin Şifresi

`internal/config/config.go` içinde admin bilgileri env değişkenleri bulunamazsa fallback olarak kullanılıyor:

```text
username: admin
password: ChangeMeNow!
```

`docker-compose.yml` içinde `CTIFEED_ADMIN_USERNAME` ve `CTIFEED_ADMIN_PASSWORD` container'a aktarılmadığı için production ortamında bu varsayılan bilgiler aktif kalabilir.

#### Öneri

Production `.env` dosyasına güçlü değerler eklenmeli:

```env
CTIFEED_ADMIN_USERNAME=gercek_admin_adi
CTIFEED_ADMIN_PASSWORD=cok_uzun_ve_rastgele_sifre
CTIFEED_ADMIN_JWT_SECRET=cok_uzun_rastgele_jwt_secret
```

Daha güvenli davranış için admin bilgileri eksikse uygulama başlatılmamalı; fallback değer kullanılmamalıdır.

### 2. HTTPS Eksikliği

Docker servisi doğrudan `8088:8080` portu üzerinden yayınlanıyor. TLS/reverse proxy yapılandırması bulunmuyor.

Bu durumda:

- Admin login bilgileri ağ üzerinde görülebilir.
- JWT cookie güvenli transport olmadan gönderilebilir.
- Trafik manipülasyonu mümkün olur.

#### Öneri

Uygulama internete doğrudan açılmamalı. Önünde Nginx, Caddy, Traefik veya cloud load balancer bulunmalı ve HTTPS zorunlu hale getirilmelidir.

### 3. Gerçek Tokenların Korunması

`.env` dosyasında Telegram ve Twitter tokenları bulunuyor. Dosya `.gitignore` ile dışlanmış olsa da tokenlar paylaşılmış veya loglanmışsa artık güvenilir kabul edilmemelidir.

Etkilenen bilgiler:

- `TELEGRAM_BOT_TOKEN`
- `TWITTER_AUTH_TOKEN`
- `TWITTER_CT0`

#### Öneri

Bu tokenlar yenilenmeli, production secret manager kullanılmalı ve tokenlar loglara yazılmamalıdır.

### 4. Veritabanı Backup Planı Yok

MySQL verisi Docker volume üzerinde tutuluyor. Otomatik backup, retention ve restore prosedürü bulunmuyor.

Volume veya sunucu kaybında şu veriler kaybedilebilir:

- Makaleler
- IoC kayıtları
- Kaynaklar
- Telegram abonelikleri
- Kaynak sağlık bilgileri

#### Öneri

- Günlük otomatik MySQL dump alınmalı.
- Backup farklı bir diskte veya object storage'da tutulmalı.
- Backup restore işlemi düzenli olarak test edilmeli.
- En az 7-30 günlük retention uygulanmalı.

## Yüksek Öncelikli Riskler

### 5. IoC Kayıt Hataları Sessizce Yok Sayılıyor

`internal/storage/mysql.go` içinde bazı IoC insert hataları `_ =` ile yok sayılıyor.

Sonuç olarak article kaydedilip IoC kaydı başarısız olabilir ve tarama başarılı görünür.

#### Öneri

IoC insert hataları:

- loglanmalı,
- transaction'ı başarısız saymalı veya
- sonuçta ayrı bir `ioc_errors` sayacı olarak raporlanmalıdır.

### 6. `INSERT IGNORE` Kullanımı

Article ve IoC kayıtlarında `INSERT IGNORE` kullanılıyor. Bu duplicate kayıtları engellese de bazı constraint ve veri hatalarını sessizce bastırabilir.

#### Öneri

Duplicate ve gerçek database hataları ayrıştırılmalı. Her taramada şu bilgiler raporlanmalı:

- yeni eklenen article sayısı
- duplicate sayısı
- başarısız kayıt sayısı
- başarısız IoC sayısı

### 7. JWT Secret Eksikse Rastgele Üretiliyor

`CTIFEED_ADMIN_JWT_SECRET` verilmezse uygulama rastgele secret oluşturuyor.

Bu durumda container yeniden başlatıldığında mevcut admin oturumları geçersiz hale geliyor.

#### Öneri

Production modunda secret eksikse uygulama başlamamalı. Secret deployment secret store veya environment secret olarak verilmelidir.

### 8. Admin Login Rate Limit İçermiyor

`POST /api/admin/login` endpoint'inde brute-force koruması bulunmuyor.

#### Öneri

- IP başına rate limit
- Başarısız deneme sayacı
- Geçici hesap/IP kilitleme
- Başarısız giriş audit logu
- Reverse proxy seviyesinde ek limit

eklenmelidir.

### 9. Logout Çalınan JWT'yi İptal Etmiyor

Logout işlemi browser cookie'sini siliyor. Ancak daha önce kopyalanmış bir JWT token, süresi dolana kadar kullanılabilir.

#### Öneri

Daha sıkı bir oturum sistemi için:

- kısa ömürlü access token
- refresh token rotation
- server-side session ID
- token revoke/blacklist
- admin şifre değişiminde tüm session'ları iptal etme

uygulanmalıdır.

### 10. Kaynak Ekleme SSRF Riski

Admin kaynak ekleyerek backend'e istediği HTTP/HTTPS URL'sini fetch ettirebilir. Admin hesabı ele geçirilirse saldırgan internal servisleri hedefleyebilir.

#### Öneri

Kaynak URL doğrulamasında şu adresler engellenmeli:

- `127.0.0.1`
- `localhost`
- `0.0.0.0`
- private IPv4 aralıkları
- link-local adresler
- Docker internal servis adresleri
- cloud metadata adresleri

Redirect sonrası hedef adres de tekrar doğrulanmalıdır.

## Orta Öncelikli Riskler

### 11. Scheduler Shutdown Context'inden Kopuk

Bazı background scan işlemleri `context.Background()` ile başlatılıyor. Uygulama kapanırken devam eden tarama iptal edilmeyebilir.

#### Öneri

Scheduler ve scan işlemleri ana shutdown context'i kullanmalı. Shutdown sırasında yeni scan başlatılmamalı ve çalışan scan için sınırlı graceful timeout uygulanmalıdır.

### 12. Migration Sistemi Versioned Değil

Database migration sistemi yalnızca `CREATE TABLE IF NOT EXISTS` sorgularından oluşuyor. Migration versiyon tablosu bulunmuyor.

Ayrıca startup sırasında bazı cleanup sorguları çalışıyor:

- Telegram IoC kayıtları siliniyor.
- Gelecekteki article tarihleri değiştiriliyor.

#### Öneri

- Migration tablosu eklenmeli.
- Her migration numaralı ve tek seferlik uygulanmalı.
- Destructive cleanup migration dışına alınmalı.
- Migration öncesi backup alınmalı.
- Rollback veya geri dönüş prosedürü hazırlanmalı.

### 13. MySQL Portu Host'a Açık

`docker-compose.yml` MySQL'i `3307:3306` ile host'a yayınlıyor.

#### Öneri

Production'da MySQL portu dışarı açılmamalı. Sadece CTIFeed container'ının bulunduğu Docker network üzerinden erişilebilir olmalı.

Ayrıca database root şifresi ve kullanıcı şifresi compose dosyasında sabit tutulmamalı.

### 14. Twitter Scraping Kırılgan

Twitter verileri resmi API yerine X HTML'i, syndication endpoint'i ve regex ile ayrıştırılıyor.

X HTML yapısını değiştirdiğinde veri toplama durabilir. `auth_token` ve `ct0` tokenları da zamanla geçersiz hale gelebilir.

#### Öneri

- Twitter fetch başarısızlıkları alarm üretmeli.
- Kaynak sağlık durumu izlenmeli.
- Parser için fixture tabanlı testler eklenmeli.
- Mümkünse resmi API veya sürdürülebilir bir provider kullanılmalı.

### 15. Integration Testleri MySQL Yoksa Skip Ediliyor

API ve storage testleri MySQL bulunamazsa `Skip` oluyor. Bu CI sisteminde testlerin hiç çalışmadan başarılı görünmesine neden olabilir.

#### Öneri

CI pipeline içinde gerçek MySQL service container kullanılmalı. Integration testler zorunlu hale getirilmeli.

### 16. Frontend Dependency Kurulumu Reproducible Değil

Dockerfile içinde `npm install` kullanılıyor.

#### Öneri

Lockfile mevcutsa:

```dockerfile
RUN npm ci
```

kullanılmalı. Ayrıca dependency audit CI pipeline'a eklenmeli.

## Operasyonel Eksikler

### Monitoring

Aşağıdaki metrikler izlenmeli:

- başarılı/başarısız kaynak sayısı
- scan süresi
- MySQL bağlantı havuzu
- yeni article sayısı
- duplicate sayısı
- IoC insert hataları
- login başarısızlıkları
- Twitter token hataları

### Health Checks

Mevcut healthcheck temel HTTP erişimini kontrol ediyor. Aşağıdakiler ayrı kontrol edilmeli:

- database bağlantısı
- son başarılı scan zamanı
- kaynak fetch hata oranı
- scheduler durumu

### Loglama

Loglarda şunlar bulunmalı:

- request ID
- scan ID
- source URL veya source ID
- duration
- inserted/skipped/error sayıları
- admin authentication olayları

Şifre, JWT, Telegram token ve Twitter tokenları kesinlikle loglanmamalıdır.

## Canlıya Alma Öncesi Kontrol Listesi

- [ ] Varsayılan admin kullanıcı adı ve şifresi değiştirildi.
- [ ] `CTIFEED_ADMIN_JWT_SECRET` uzun rastgele değer olarak tanımlandı.
- [ ] Telegram token yenilendi veya güvenli secret store'a taşındı.
- [ ] Twitter tokenları güvenli şekilde tanımlandı.
- [ ] HTTPS reverse proxy kuruldu.
- [ ] MySQL portu dış dünyaya kapatıldı.
- [ ] MySQL backup otomasyonu kuruldu.
- [ ] Backup restore testi yapıldı.
- [ ] Login rate limit eklendi.
- [ ] SSRF URL doğrulaması eklendi.
- [ ] IoC kayıt hataları görünür hale getirildi.
- [ ] Migration versioning eklendi.
- [ ] CI içinde gerçek MySQL integration test çalışıyor.
- [ ] Frontend dependency audit çalışıyor.
- [ ] Healthcheck ve monitoring kuruldu.
- [ ] Error tracking kuruldu.
- [ ] Production deploy rollback planı hazırlandı.

## Mevcut Doğrulama

Çalıştırılan kontroller:

```text
go vet ./...
go test -count=1 ./...
```

Go static analysis ve tüm Go testleri başarılı geçti.

Frontend build başarılıdır. `npm audit` kontrolü PowerShell execution policy nedeniyle bu incelemede çalıştırılamadı.

## Sonuç

CTIFeed'in temel işlevleri ve admin JWT akışı çalışır durumdadır. Ancak canlıya açılmadan önce özellikle secret yönetimi, HTTPS, backup, database erişimi, login rate limit ve IoC hata takibi tamamlanmalıdır.

Bu maddeler çözülmeden uygulamanın doğrudan public internete açılması önerilmez.
