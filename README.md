# QuizBattle

QuizBattle لعبة معرفة عربية تنافسية مستوحاة من روح **سيف المعرفة**: كل سؤال يتحول إلى بطاقة قابلة للجمع، ويختار كل لاعب خمس بطاقات لساحات فردية أو جماعية يصل عددها إلى 8 لاعبين.

> **الحالة الحالية:** MVP قابل للنشر بإصدار إنتاجي أحادي النسخة ومؤمّن عبر Docker وGitHub Actions وCloudflare. يعمل الحساب، اللوبي العام، الدردشة، محرك الجولات الخادمي، البطاقات، العملات، السوق والتبادل، مع قيود التوسع والصوت الموضحة أدناه.

## ما يعمل الآن

| المجال | التنفيذ الحالي |
| --- | --- |
| الحسابات | تسجيل، دخول، جلسة HttpOnly، إلغاء جلسة عند الخروج، وتحديث الاسم وتاريخ الميلاد وصورة اللاعب |
| مجتمع اللاعبين | لوبي عام ومجلس محادثة محفوظ في MongoDB بهوية يثبتها الخادم؛ آخر 50 رسالة تعود بعد التحديث |
| صوت الساحة | صوت WebRTC اختياري مباشر في 1 ضد 1، مع كتم ومغادرة ومن دون تسجيل الصوت؛ الجماعي يحتاج SFU قبل الإنتاج |
| المباريات | 1 ضد 1، 2 ضد 2، 4 ضد 4، أو ساحة مفتوحة من 2 إلى 8؛ 20 ثانية للسؤال وكشف نتيجة 3 ثوانٍ |
| بنك الأسئلة | 1,573 سؤالًا عربيًا في 9 مجالات، مع مصادر وبصمات محتوى ومدقق مستقل |
| البطاقات | 10 بطاقات بداية لكل لاعب، ندرة وقوة وسجل لعب/فوز، وتجهيز تلقائي قابل للمراجعة، وحجز ذري أثناء المباراة |
| العملات | 600 عملة بداية؛ البطل 120، زميل الفريق الفائز 90، والخاسر 45؛ لا مكافأة للانسحاب |
| السوق | عرض بطاقة، شراء ذري، إلغاء، رسم 5%، ومنع البيع المزدوج |
| التبادل | بطاقة/عملات مقابل بطاقة/عملات، قبول/رفض/إلغاء، وضمان ذري |
| الهاتف | واجهة عربية RTL محلية بالكامل، شريط هاتف ثابت، ومقاسات تبدأ من 320px |
| التشغيل | Docker Compose، MongoDB replica set، health/readiness، graceful shutdown وفهارس |

الساحات الخاصة، الدعوات، البوتات، الترتيب الموسمي، والمشاهدة ليست ضمن هذا الـMVP بعد.

## قواعد الـMVP

- الأنماط الثابتة هي `duel` (لاعبان)، `team_2v2` (4 لاعبين)، و`team_4v4` (8 لاعبين). النمط `open` يقبل من 2 إلى 8 وفق الحد الذي يختاره المالك.
- يضغط المالك «تجهيز الساحة» بعد اكتمال العدد المطلوب؛ عندها تتجمد العضوية ولا يعود الانضمام ممكنًا.
- يمكن لكل مالك تشغيل 3 ساحات متزامنة كحد أقصى؛ الساحات المكتملة أو المنتهية بالانسحاب تبقى في السجل ولا تُحتسب ضمن الحد.
- يحصل كل حساب جديد على 10 بطاقات و600 عملة.
- يثبت كل لاعب 5 بطاقات مملوكة ومتاحة؛ هذا هو إعلان الجاهزية وتدخل البطاقات في حجز المباراة.
- يستطيع اللاعب طلب «تجهيز تلقائي» يرتب بطاقاته بالقوة ثم الندرة ونسبة الفوز؛ الاقتراح لا يُعتمد تلقائيًا، ويمكن تعديله أو استبداله يدويًا قبل التثبيت.
- لا يستطيع إلا مالك الساحة البدء، ولا يتاح الزر حتى يصبح جميع اللاعبين جاهزين.
- عدد الأدوار الرئيسي هو `5 × عدد اللاعبين`؛ كل لاعب مؤهل يجيب مرة واحدة خلال 20 ثانية وفق ساعة الخادم.
- الإجابة الصحيحة تمنح 100 نقطة، وتصل مكافأة السرعة إلى 50 نقطة إضافية.
- لا تُرسل الإجابة الصحيحة أو الشرح إلى المتصفح قبل إغلاق الدور.
- قوة البطاقة وندرتها للهوية والتقدم، وليستا مضاعفًا للنقاط حتى لا تصبح اللعبة مدفوعة للفوز.
- في الساحة المفتوحة يفوز أعلى لاعب نقاطًا. في 2 ضد 2 و4 ضد 4 يفوز أعلى فريق بمجموع نقاط أعضائه، ثم يُختار صاحب أعلى نقاط داخله بطلًا نهائيًا.
- إذا تعادل المتصدرون تبدأ أسئلة حسم محايدة للمتعادلين فقط، وتتكرر حتى يظهر بطل واحد؛ لا يعلن المحرك تعادلًا نهائيًا ما دام بنك الحسم متاحًا.
- لا تنتقل بطاقة قسرًا بسبب نتيجة مباراة؛ انتقال الملكية يحدث فقط عبر السوق أو تبادل صريح.
- الانسحاب ينهي المباراة بلا مكافآت ويحرر جميع البطاقات المحجوزة.

العقد التفصيلي وحدود المجالات موجودان في [docs/mvp-domain-design.md](docs/mvp-domain-design.md)، ومواصفة التصميم في [docs/card-design-spec.md](docs/card-design-spec.md).

## بنك الأسئلة

الملف المعتمد هو [data/question-bank/questions.ar.jsonl](data/question-bank/questions.ar.jsonl):

| المجال | العدد |
| --- | ---: |
| رياضيات | 500 |
| جغرافيا | 314 |
| علوم | 236 |
| مدن | 157 |
| معرفة دينية | 114 |
| تقنية | 86 |
| سياسة ومدنيات | 68 |
| ثقافة عامة | 54 |
| تاريخ | 44 |
| **الإجمالي** | **1,573** |

كل سجل يحتوي أربعة خيارات فريدة، إجابة، شرحًا، مصدرًا، تاريخ تحقق، وحقل `contentHash`. المدقق يرفض تكرار المعرّفات أو نصوص الأسئلة والأشكال غير الصحيحة. مصادر البيانات الأساسية موثقة في [data/question-bank/SOURCES.md](data/question-bank/SOURCES.md)، وتشمل Wikidata وIANA وBIPM وIUPAC وUN M49 وQuran Foundation. أسئلة المعرفة الدينية مقتصرة على أسماء السور وترتيبها، وتحتاج مراجعة بشرية متخصصة قبل إطلاق عام.

للتحقق:

```bash
python tools/question-bank/validate.py --minimum 1000
python -m unittest discover -s tools/question-bank -p "test*.py"
```

## المعمارية

```text
Browser (Arabic HTML/CSS/JavaScript)
  ├─ REST commands/queries ─> middleware ─> controllers ─> application services
  ├─ WebSocket events/chat/signaling ─> authenticated bounded registry/hubs
  ├─ World-chat history    ─> bounded Mongo retention (100 messages / 7 days)
  ├─ Arena audio           ─> WebRTC peer-to-peer (server relays signaling only)
  └─ safe match snapshots  <─ domain aggregates ─> transactional Mongo repositories

Domain modules: match | economy | question
Mongo collections: users, Game, Matches, QuestionBank, Cards, Wallets,
                   MarketListings, TradeOffers, EconomyLedger, SessionRevocation,
                   ChatMessage, UserAvatar
```

التطبيق modular monolith بلغة Go. المعاملات التي تغيّر عملات أو ملكية بطاقات تتطلب MongoDB replica set. أحداث الوقت الحقيقي ومحددات المعدل ما زالت داخل العملية، لذلك شغّل **نسخة تطبيق واحدة فقط** حتى إضافة broker وحالة موزعة.

## التشغيل المحلي

### Docker Compose

1. انسخ `.env.example` إلى `.env`.
2. أنشئ قيمة عشوائية جديدة لـ`JWT_SECRET` بطول 32 حرفًا على الأقل؛ المثال يتركها فارغة عمدًا.
3. شغّل:

```bash
docker compose up --build
```

افتح `http://127.0.0.1:8080`. ينشئ Compose MongoDB replica set أحادي العقدة، ويستورد بنك الأسئلة عندما تكون `SEED_DATABASE=true`.

### تشغيل Go مباشرة

المتطلبات: Go 1.26.6 وMongoDB replica set. من مجلد `src`:

```bash
go mod download
go run .
```

لـHTTP المحلي استخدم `APP_ENV=development` و`COOKIE_SECURE=false`. كل بيئة أخرى تفرض Secure cookie وMongoDB TLS موثوق الشهادة.

## اختبار محلي كامل

بعد تشغيل البيئة المحلية، ينشئ الأمر التالي حسابات محلية فريدة ويختبر حفظ الدردشة، وإشارات WebRTC المصادق عليها من دون تشغيل ميكروفون، ودورة 1 ضد 1 كاملة، ثم تجهيز وبدء 2 ضد 2 و4 ضد 4 وساحة مفتوحة، إضافة إلى المكافآت والسوق والتبادل وتحرير البطاقات بعد الانسحاب:

```bash
cd src
go run ./cmd/e2e -base http://127.0.0.1:8080
```

يرفض الأمر أي مضيف غير loopback افتراضيًا حتى لا ينشئ بيانات اختبار في بيئة بعيدة بالخطأ.

## الإعدادات

| المتغير | مطلوب | الافتراضي | الغرض |
| --- | --- | --- | --- |
| `MONGO_URI` | نعم | — | MongoDB URI؛ TLS موثوق مطلوب خارج development/test |
| `MONGO_DATABASE` | نعم | — | اسم قاعدة البيانات |
| `JWT_SECRET` | نعم | — | مفتاح عشوائي خاص بكل نشر، 32 حرفًا على الأقل |
| `APP_ENV` | لا | `production` | استخدم `development` أو `test` فقط محليًا |
| `RELEASE_SHA` | في production/staging | — | رقم commit كامل: 40 حرفًا hexadecimal صغيرًا؛ يظهر في health checks لمنع نشر إصدار خاطئ |
| `PORT` | لا | `8080` | منفذ HTTP |
| `SESSION_TTL` | لا | `1h` | مدة الجلسة بين 15 دقيقة و24 ساعة |
| `COOKIE_SECURE` | لا | حسب البيئة | لا يمكن تعطيله خارج development/test |
| `ALLOWED_ORIGINS` | لا | نفس المضيف | origins إضافية لفحص write/WS؛ لا يفعّل CORS |
| `TRUSTED_PROXY_CIDRS` | لا | فارغ | CIDRs الفعلية للـload balancer فقط |
| `SEED_DATABASE` | لا | `false` | استيراد بنك الأسئلة idempotently عند الإقلاع |
| `QUESTION_BANK_PATH` | لا | `../data/question-bank/questions.ar.jsonl` | مسار ملف JSONL الداخلي |
| `REDIS_ADDRESS` | لا | فارغ | Redis اختياري قبل الأحداث الموزعة |
| `REDIS_USERNAME` | لا | فارغ | Redis ACL username |
| `REDIS_PASSWORD` | لا | فارغ | Redis password |
| `REDIS_TLS` | لا | `false` | مطلوب عند استخدام Redis خارج development/test |

لا تدعم الشفرة fallback باسم `ACCESS_SECRET`. لا تحفظ `.env` أو مفاتيح سحابية أو JWT أو CI tokens في Git.

## واجهات HTTP وWebSocket

Public:

- `GET /`, `/about`, `/contact`, `/auth/signin`, `/auth/signup`
- `POST /user/createuser`, `POST /user/login`, `POST /user/logout`
- `GET /healthz` — يعيد `{"status":"ok","release":"<commit-sha>"}`، و`GET /readyz`

Authenticated account/lobby:

- `GET /api/v1/session`, `POST /api/v1/user`
- `PUT /api/v1/user/avatar`, `DELETE /api/v1/user/avatar`
- `GET /api/v1/user/avatar/{id}` — صورة JPEG معالجة، خاصة ومحمية بالجلسة
- `GET /api/v1/chat/messages` — أحدث 50 رسالة محفوظة، تصاعديًا
- `POST /api/v1/game`
- `POST /api/v1/game/{id}/join`, `POST /api/v1/game/{id}/exit`
- `GET /api/v1/game/{id}`, `GET /api/v1/games/public`, `GET /api/v1/games/mine`

Authenticated match:

- `POST /api/v1/game/{id}/prepare` — المالك يجمّد المشاركين ويفتح تجهيز البطاقات
- `PUT /api/v1/game/{id}/deck`
- `POST /api/v1/game/{id}/start`
- `GET /api/v1/game/{id}/match`
- `POST /api/v1/game/{id}/answer`
- `POST /api/v1/game/{id}/forfeit`

Authenticated economy:

- `GET /api/v1/collection`
- `GET /api/v1/market`, `POST /api/v1/market/listings`
- `POST /api/v1/market/listings/{id}/buy|cancel`
- `GET /api/v1/trades`, `POST /api/v1/trades`
- `POST /api/v1/trades/{id}/accept|reject|cancel`

Realtime:

- `GET /ws/events`, `GET /ws/world-chat`, `GET /ws/game/{id}`

رسالة المجلس تُحفظ قبل بثها، ويحتفظ الخادم بآخر 100 رسالة لمدة أقصاها سبعة أيام. قناة الساحة تقبل أحداث WebRTC المحددة فقط (`voice_ready`, `voice_leave`, `voice_offer`, `voice_answer`, `voice_ice`) وتعيد كتابة هوية المرسل من الجلسة؛ الصوت نفسه لا يمر بالخادم ولا يُسجل. طلب الميكروفون لا يحدث إلا بعد ضغط اللاعب على زر الانضمام.

لا يوجد raw question endpoint. الخادم يختار السؤال من البطاقة المثبتة، ويعيد snapshot مختلفًا لكل لاعب. المعرّفات العامة ذات 64 بت تُرسل كسلاسل JSON حتى لا تفقد JavaScript دقتها.

## الجودة والأمان

من `src`:

```bash
go fmt ./...
go vet ./...
go test -count=1 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
go run github.com/securego/gosec/v2/cmd/gosec@v2.28.0 -quiet ./...
```

راجع [SECURITY.md](SECURITY.md)، [تقرير المراجعة](docs/review-report-2026-08-15.md)، و[ملاحظات الهجرة](docs/migration-notes.md) قبل أي نشر.

## حدود التشغيل الإنتاجي

- أُلغيت مفاتيح Firebase وCoveralls التاريخية، ويُولّد النشر JWT secret جديدًا. أُعيدت كتابة الفروع القابلة للتعديل وفحص التاريخ الكامل (206 commits) بلا تسريبات؛ يظل طلب إزالة مراجع PR والنسخ المخبأة القديمة لدى GitHub Support إجراء تنظيف متابعة.
- أبقِ نسخة تطبيق واحدة؛ لا autoscaling ولا zero-downtime overlap قبل توزيع WebSocket state/events/rate limits عبر broker مشترك.
- تحفظ أسرار التشغيل في ملفات root-only على الخادم، وMongoDB داخلي مع TLS ومصادقة، وتُشفّر النسخ الاحتياطية. انقل النسخ المشفرة ومفتاح الاستعادة إلى موقع منفصل واختبر الاستعادة دوريًا.
- خط الإنتاج يفحص الاختبارات وrace detector و`govulncheck` و`gosec` وتاريخ الأسرار، ويبني image ثابتة بالـdigest مع SBOM واختبار topology إنتاجي قبل النشر.
- جهّز TURN بإعتمادات قصيرة العمر صادرة من الخادم قبل اعتبار الصوت مضمونًا على كل الشبكات؛ STUN وحده لا يضمن الاتصال عبر الشبكات المقيدة.

Licensed under the [MIT License](LICENSE).
