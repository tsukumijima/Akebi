
# Akebi

💠 **Akebi:** **A** **ke**yless https server, and **b**ackend dns server that resolves **i**p from domain

> Sorry, the documentation is currently in Japanese only. Google Translate is available.

インターネットに公開されていないプライベート Web サイトを「正規」の Let’s Encrypt の証明書で HTTPS 化するための、HTTPS リバースプロキシサーバーです。

この HTTPS リバースプロキシサーバーは、

- **権威 DNS サーバー:** `192-168-1-11.local.example.com` のようにサブドメインとして IP アドレスを指定すると、そのまま `192.168.1.11` に名前解決するワイルドカード DNS
- **API サーバー:** 事前に Let’s Encrypt で取得した証明書と秘密鍵を保持し、TLS ハンドシェイク時の証明書の供給と、Pre-master Secret Key の生成に使う乱数に秘密鍵でデジタル署名を行う API
- **デーモンプロセス:** Let’s Encrypt で取得した *.local.example.com の HTTPS ワイルドカード証明書と、API サーバーの HTTPS 証明書を定期的に更新するデーモン

の3つのコンポーネントによって構成される、**Keyless Server** に依存しています。

以下、HTTPS リバースプロキシサーバーを **HTTPS Server** 、上記の3つの機能を持つバックエンドサーバーを **Keyless Server** と呼称します。

Keyless Server のコードの大半と HTTPS Server の TLS ハンドシェイク処理は、[ncruces](https://github.com/ncruces) さん開発の [keyless](https://github.com/ncruces/keyless) をベースに、個人的な用途に合わせてカスタマイズしたものです。  
偉大な発明をしてくださった ncruces さんに、この場で心から深く感謝を申し上げます（私が書いたコードは 20% 程度にすぎません）。

## 開発背景

**Akebi は、オレオレ証明書以外での HTTPS 化が困難なローカル LAN 上でリッスンされるサーバーアプリケーションを、Let's Encrypt 発行の正規の HTTPS 証明書で HTTPS 化するために開発されました。**

-----

ローカル LAN やイントラネットなどのプライベートネットワークでリッスンされている Web サーバーは、HTTP でリッスンされていることがほとんどです。

これは盗聴されるリスクが著しく低く、VPN 経由なら元々暗号化されているなどの理由で HTTPS にする必要がないこと、プライベートネットワークで信頼される HTTPS 証明書の入手が事実上難しいことなどが理由でしょう。HTTP の方が単純で簡単ですし。

### ブラウザの HTTPS 化の圧力

…ところが、最近のブラウザはインターネット上に公開されている Web サイトのみならず、**盗聴のリスクが著しく低いプライベートネットワーク上の Web サイトにも、HTTPS を要求するようになってきました。**

すでに PWA の主要機能である Service Worker や Web Push API などをはじめ、近年追加された多くの Web API の利用に（中には WebCodecs API のような HTTPS 化を必須にする必要が皆無なものも含めて）**HTTPS が必須になってしまっています。**

> [!NOTE]  
> 正確には **[安全なコンテキスト (Secure Contexts)](https://developer.mozilla.org/ja/docs/Web/Security/Secure_Contexts)** でないと動作しないようになっていて、特別に localhost (127.0.0.1) だけは http:// でも安全なコンテキストだと認められるようになっています。

プライベート Web サイトであっても、たとえばビデオチャットのために [getUserMedia()](https://developer.mozilla.org/ja/docs/Web/API/MediaDevices/getUserMedia) を、クリップボードにコピーするために [Clipboard API](https://developer.mozilla.org/ja/docs/Web/API/Clipboard_API) を使いたい要件が出てくることもあるでしょう（どちらも Secure Contexts が必須です）。  

- せっかくコードは Service Worker に対応しているのに、HTTP では Service Worker が動かないのでキャッシュが効かず、読み込みがたびたび遅くなる
- PWA で Android のホーム画面にインストールしてもアイコンが Chrome 扱いになるし、フォームに入力すると上部に「保護されていない通信」というバナーが表示されてうざい
- Clipboard API・Storage API・SharedArrayBuffer などの強力な API が Secure Contexts でないと使えず、今後の機能開発が大きく制約される

私が開発している [KonomiTV](https://github.com/tsukumijima/KonomiTV) でも、上記のような課題を抱えていました。  

しかも、最近新たに追加された API はその性質に関わらず問答無用で [Secure Contexts が必須になっている](https://developer.mozilla.org/ja/docs/Web/Security/Secure_Contexts/features_restricted_to_secure_contexts) ことが多く、リッチなプライベート Web サイトの開発はかなりやりづらくなってきています。

さらに、Chrome 94 から適用された [Private Network Access](https://developer.chrome.com/blog/private-network-access-update/) という仕様のおかげで、**HTTP の公開 Web サイトからプライベート Web サイトにアクセスできなくなりました。** CORS ヘッダーで明示的に許可していても、です。

以前より HTTPS の公開 Web サイトから HTTP のプライベート Web サイトへのアクセスは、Mixed Content として禁止されています (localhost を除く) 。そのため、公開 Web サイトも HTTP (Public (HTTP) -> Private (HTTP)) の構成にせざるを得なかったのですが、それすらも禁止されてしまいました。

こうした変更は、公開 Web サイトからローカル LAN 上にあるデバイスを操作する類のアプリケーションにとって、かなり厳しい制約になります。

> [!NOTE]  
> Chrome 105 以降では、Public (HTTPS) -> Private (HTTPS) のアクセスには、さらにプライベート Web サイト側のレスポンスに `Access-Control-Allow-Private-Network` ヘッダーを付与する必要があるようです ([参考](https://developer.chrome.com/blog/private-network-access-preflight/))。  
> Chrome 105 以降も公開 Web サイトからプライベート Web サイトにアクセスするには両方の HTTPS 化が必須で、加えて Preflight リクエストが飛んできたときに `Access-Control-Allow-Private-Network: true` を返せる必要が出てきます。

### プライベート Web サイトの証明書取得の困難さ

一般的な公開 Web サイトなら、Let's Encrypt を使うことで無料で簡単に HTTPS 化できます。無料で HTTPS 証明書を取れるようになったこともあり、ブラウザによる HTTPS 化の圧力は年々強まっています。

しかし、プライベート Web サイトの場合、**正攻法での HTTPS 化は困難を極めます。**  
当然インターネット上からは Web サーバーにアクセスできないため、Let's Encrypt の HTTP-01 チャレンジが通りません。  
…それ以前に Let's Encrypt は元々 IP アドレス宛には証明書を発行できませんし、グローバル IP ならまだしも、世界各地で山ほど被りまくっているプライベート IP の所有権を主張するのには無理があります。

そこでよく利用されるのが、**自己署名証明書（オレオレ証明書）を使った HTTPS 化**です。

自分で HTTPS 証明書を作ってしまう方法で、プライベート IP アドレスだろうが関係なく、自由に証明書を作成できます。  
最近では [mkcert](https://github.com/FiloSottile/mkcert) のような、オレオレ証明書をかんたんに生成するツールも出てきています。

自分で作った証明書なので当然ブラウザには信頼されず、そのままではアクセスすると警告が表示されてしまいます。  
ブラウザに証明書を信頼させ「この接続ではプライバシーが保護されません」の警告をなくすには、**生成したオレオレ証明書を OS の証明書ストアに「信頼されたルート証明機関」としてインストールする必要があります。**

mkcert はそのあたりも自動化してくれますが、それはあくまで開発時の話。  
mkcert をインストールした PC 以外のデバイスには手動でインストールしないといけませんし、インストール方法もわりと面倒です。開発者ならともかく、一般ユーザーには難易度が高い作業だと思います。  
しかも、プライベート Web サイトを閲覧するデバイスすべてにインストールしなければならず、閲覧デバイスが多ければ多いほど大変です。

…こうした背景から、**一般ユーザーに配布するアプリケーションでは、事実上オレオレ証明書は使えない状態です。**  
もちろんユーザー体験を犠牲にすれば使えなくはありませんが、より多くの方に簡単に使っていただくためにも、できるだけそうした状態は避けたいです。

### Let's Encrypt の DNS 認証 + ワイルドカード DNS という選択肢

閑話休題。オレオレ証明書に押されてあまり知られていないのですが、**実はプライベート Web サイトでも、Let's Encrypt の DNS 認証 (DNS-01 チャレンジ) を使えば、正規の HTTPS 証明書を取ることができます。**  
詳細は [この記事](https://blog.jxck.io/entries/2020-06-29/https-for-localhost.html) が詳しいですが、軽く説明します。

通常、DNS 上の A レコードにはグローバル IP アドレスを指定します。ですが、とくにグローバル IP アドレスでないといけない制約があるわけではありません。`127.0.0.1` や `192.168.1.1` を入れることだって可能です。

たとえば、`local.example.com` の A レコードを `127.0.0.1` に設定したとします。もちろんループバックアドレスなのでインターネット上からはアクセスできませんし、Let's Encrypt の HTTP 認証は通りません。

そこで、**Let's Encrypt の DNS 認証 (DNS-01 チャレンジ) で HTTPS 証明書を取得します。**  
DNS 認証は、例でいう `local.example.com` の DNS を変更できる権限（≒ドメインの所有権）を証明することで、HTTPS 証明書を取得する方法です。  
DNS 認証ならインターネットからアクセスできる必要はなく、**DNS 認証時に `_acme-challenge.local.example.com` の TXT レコードにトークンを設定できれば、あっさり HTTPS 証明書が取得できます。**

……一見万事解決のように見えます。が、この方法はイントラネット上のサイトなどでプライベート IP アドレスが固定されている場合にはぴったりですが、**不特定多数の環境にインストールされるプライベート Web サイトでは、インストールされる PC のプライベート IP アドレスが環境ごとにバラバラなため、そのままでは使えません。**

**そこで登場するのがワイルドカード DNS サービスです。**[nip.io](https://nip.io/) や [sslip.io](https://sslip.io/) がよく知られています。  
これらは **`http://192-168-1-11.sslip.io` のようなサブドメインを `192.168.1.11` に名前解決してくれる特殊な DNS サーバー**で、sslip.io の方は自分が保有するドメインをワイルドカード DNS サーバーにすることもできます。

また、**実は Let's Encrypt ではワイルドカード証明書を取得できます。** ドメインの所有権を証明できれば、`hoge.local.example.com`・`fuga.local.example.com`・`piyo.local.example.com` いずれでも使える証明書を発行できます。

このワイルドカード DNS サービスと取得したワイルドカード証明書を組み合わせれば、**`http://192.168.1.11:3000/` の代わりに `https://192-168-1-11.local.example.com:3000/` にアクセスするだけで、魔法のように正規の証明書でリッスンされるプライベート HTTPS サイトができあがります！**

> [!NOTE]  
> 『ワイルドカード DNS と Let's Encrypt のワイルドカード証明書を組み合わせてローカル LAN で HTTPS サーバーを実現する』というアイデアは、[Corollarium](https://github.com/Corollarium) 社開発の [localtls](https://github.com/Corollarium/localtls) から得たものです。

### 証明書と秘密鍵の扱い

経緯の説明がたいへん長くなってしまいましたが、ここからが本番です。

上記の手順を踏むことで、プライベート Web サイトでも HTTPS 化できる道筋はつきました。  
ですが、不特定多数の環境にインストールされるプライベート Web サイト（そう多くはないが、著名な例だと Plex Media Server などの一般ユーザーに配布されるアプリケーションが該当する）では、**HTTPS 証明書・秘密鍵の扱いをどうするかが問題になります。**

アプリケーション自体を配布しなければならないので、当然証明書と秘密鍵もアプリケーションに同梱しなければなりません。ですが、このうち秘密鍵が漏洩すると、別のアプリケーションがなりすましできたり、通信を盗聴できたりしてしまいます（中間者攻撃）。

もっとも今回はブラウザへの建前として形式上 HTTPS にしたいだけなのでその点は正直どうでもいいのですが、それよりも **「証明書と秘密鍵があれば誰でも HTTPS 証明書を失効できてしまう」「秘密鍵の公開は Let's Encrypt の利用規約で禁止されている」点が厄介です。**

アプリケーションの内部に秘密鍵を隠すこともできますが、所詮は DRM のようなもので抜本的とはいえないほか、OSS の場合は隠すこと自体が難しくなります。  
また、Let's Encrypt 発行の HTTPS 証明書は3ヶ月で有効期限が切れるため、各環境にある証明書・秘密鍵をどうアップデートするかも問題になります。

**この「秘密鍵の扱いをどうするか」問題を、TLS ハンドシェイクの内部処理をハックし秘密鍵をリモートサーバーに隠蔽することで解決させた点が、Akebi HTTPS Server の最大の特徴です。**

> [!NOTE]  
> 証明書も TLS ハンドシェイク毎に Keyless Server からダウンロードするため、保存した証明書の更新に悩む必要がありません。

秘密鍵をリモートサーバーに隠蔽するためには、TLS ハンドシェイク上で秘密鍵を使う処理を、サーバー上で代わりに行う API サーバーが必要になります。  
**どのみち API サーバーが要るなら、sslip.io スタイルのワイルドカード DNS と Let's Encrypt の証明書自動更新までまとめてやってくれる方が良いよね？ということで開発されたのが、[ncruces](https://github.com/ncruces) さん開発の [keyless](https://github.com/ncruces/keyless) です。**

私がこの keyless をもとに若干改良したものが Akebi Keyless Server で、Akebi HTTPS Server とペアで1つのシステムを構成しています。

> [!NOTE]  
> HTTPS リバースプロキシの形になっているのは、**HTTPS 化対象のアプリケーションがどんな言語で書かれていようと HTTP サーバーのリバースプロキシとして挟むだけで HTTPS 化できる汎用性の高さ**と、**そもそも TLS ハンドシェイクの深い部分の処理に介入できるのが Golang くらいしかなかった**のが理由です。  
> 詳細は [HTTPS リバースプロキシというアプローチ](#https-リバースプロキシというアプローチ) の項目で説明しています。

## 導入

### 必要なもの

- Linux サーバー (VM・VPS)
  - Keyless Server を動かすために必要です。
  - Keyless Server は UDP 53 ポート (DNS) と TCP 443 ポート (HTTPS) を使用します。
    - それぞれ外部ネットワークからアクセスできるようにファイアウォールを設定してください。
  - Keyless Server がダウンしてしまうと、その Keyless Server に依存する HTTPS Server も起動できなくなります。安定稼働のためにも、Keyless Server は他のサイトと同居させないことをおすすめします。
  - サーバーは低スペックなものでも大丈夫です。私は [Oracle Cloud Free Tier](https://www.oracle.com/jp/cloud/free/) の AMD インスタンスで動かしています。
  - Ubuntu 20.04 LTS で動作を確認しています。
- 自分が所有するドメイン
  - Keyless Server のワイルドカード DNS 機能と、API サーバーのドメインに利用します。
  - ワイルドカード DNS 機能用のドメインは、たとえば `example.net` を所有している場合、`local.example.net` や `ip.example.net` などのサブドメインにすると良いでしょう。
    - IP → ドメインのための専用のドメインを用意できるなら、必ずしもサブドメインである必要はありません。
    - この例の場合、`192-168-1-11.local.example.net` が 192.168.1.11 に名前解決されるようになります。
  - もちろん、所有しているドメインの DNS 設定を変更できることが前提です。

### Keyless Server のセットアップ

以下は Ubuntu 20.04 LTS でのインストール手順です。

#### Golang のインストール

Go 1.18 で開発しています。

```bash
$ sudo add-apt-repository ppa:longsleep/golang-backports
$ sudo apt install golang
```

#### systemd-resolved を止める

ワイルドカード DNS サーバーを動かすのに必要です（53番ポートがバッティングするため）。  
他にもっとスマートな回避策があるかもしれないので、参考程度に…。

```bash
$ sudo systemctl disable systemd-resolved
$ sudo systemctl stop systemd-resolved
$ sudo mv /etc/resolv.conf /etc/resolv.conf.old  # オリジナルの resolv.conf をバックアップ
$ sudo nano /etc/resolv.conf
---------------------------------------------
nameserver 1.1.1.1 1.0.0.1  # ← nameserver を 127.0.0.53 から変更する
(以下略)
---------------------------------------------
```

#### DNS 設定の変更

ここからは、Keyless Server を立てたサーバーに割り当てるドメインを **`akebi.example.com`** 、ワイルドカード DNS で使うドメインを **`local.example.com`** として説明します。

`example.com` の DNS 設定で、`akebi.example.com` の A レコードに、Keyless Server を立てたサーバーの IP アドレスを設定します。IPv6 用の AAAA レコードを設定してもいいでしょう。

次に、`local.example.com` の NS レコードに、ネームサーバー（DNSサーバー）として `akebi.example.com` を指定します。  
この設定により、`192-168-1-11.local.example.com` を `192.168.1.11` に名前解決するために、`akebi.example.com` の DNS サーバー (UDP 53 番ポート) に DNS クエリが飛ぶようになります。  

#### インストール

```bash
$ sudo apt install make  # make が必要
$ git clone git@github.com:tsukumijima/Akebi.git
$ cd Akebi
$ make build-keyless-server  # Keyless Server をビルド
$ cp ./example/akebi-keyless-server.json ./akebi-keyless-server.json  # 設定ファイルをコピー
```

`akebi-keyless-server.json` が設定ファイルです。JSONC (JSON with comments) で書かれています。  
実際に変更が必要な設定は4つだけです。

- `domain`: ワイルドカード DNS で使うドメイン（この例では `local.example.com`）を設定します。
- `nameserver`: `local.example.com` の NS レコードに設定したネームサーバー（この例では `akebi.example.com`）を設定します。
- `is_private_ip_ranges_only`: ワイルドカード DNS の名前解決範囲をプライベート IP アドレスに限定するかを設定します。
  - この設定が true のとき、たとえば `192-168-1-11.local.example.com` や `10-8-0-1.local.example.com` は名前解決されますが、`142-251-42-163.local.example.com` は名前解決されず、ドメインが存在しない扱いになります。
  - プライベート IP アドレスの範囲には [Tailscale](https://tailscale.com/) の IP アドレス (100.64.0.0/10, fd7a:115c:a1e0:ab12::/64) も含まれます。
  - グローバル IP に解決できてしまうと万が一フィッシングサイトに使われないとも限らない上、用途上グローバル IP に解決できる必要性がないため、個人的には true にしておくことをおすすめします。
- `keyless_api.handler`: Keyless API サーバーの URL（https:// のような URL スキームは除外する）を設定します。
  - `akebi.example.com/` のように指定します。末尾のスラッシュは必須です。

#### セットアップ

```bash
$ sudo ./akebi-keyless-server setup
```

セットアップスクリプトを実行します。  
セットアップ途中で DNS サーバーと HTTP サーバーを起動しますが、1024 番未満のポートでのリッスンには root 権限が必要なため、sudo をつけて実行します。

```
Running setup...

Creating a new Let's Encrypt account...
Creating a new account private key...

Accept Let's Encrypt ToS? [y/n]: y
Use the Let's Encrypt production API? [y/n]: y
Enter an email address: yourmailaddress@example.com

Creating a new master private key...

Starting DNS server for domain validation...
Please, ensure that:
 - NS records for local.example.com point to akebi.example.com
 - akebi-keyless-server is reachable from the internet on UDP akebi.example.com:53
Continue? y

Obtaining a certificate for *.local.example.com...
Creating a new Keyless API private key...

Starting HTTPS server for hostname validation...
Please, ensure that:
 - akebi-keyless-server is reachable from the internet on TCP akebi.example.com:443
Continue?
Obtaining a certificate for akebi.example.com...

Done!
```

```bash
$ sudo chown -R $USER:$USER ./
```

終わったら、root 権限で作られたファイル類の所有者を、ログイン中の一般ユーザーに設定しておきましょう。  
**これで Keyless Server を起動できる状態になりました！**

certificates/ フォルダには、Let's Encrypt から取得した HTTPS ワイルドカード証明書/秘密鍵と、API サーバーの HTTPS 証明書/秘密鍵が格納されています。  
letsencrypt/ フォルダには、Let's Encrypt のアカウント情報が格納されています。

#### Systemd サービスの設定

Keyless Server は Systemd サービスとして動作します。  
Systemd に Keyless Server サービスをインストールし、有効化します。

```bash
# サービスファイルをコピー
$ sudo cp ./example/akebi-keyless-server.service /etc/systemd/system/akebi-keyless-server.service

# /home/ubuntu/Akebi の部分を Akebi を配置したディレクトリのパスに変更する
$ sudo nano /etc/systemd/system/akebi-keyless-server.service

# ソケットファイルをコピー
$ sudo cp ./example/akebi-keyless-server.socket /etc/systemd/system/akebi-keyless-server.socket

# サービスを有効化
$ sudo systemctl daemon-reload
$ sudo systemctl enable akebi-keyless-server.service
$ sudo systemctl enable akebi-keyless-server.socket

# サービスを起動
# akebi-keyless-server.socket は自動で起動される
$ sudo systemctl start akebi-keyless-server.service
```

**`https://akebi.example.com` にアクセスして 404 ページが表示されれば、Keyless Server のセットアップは完了です！** お疲れ様でした。

**Keyless Server が起動している間、Let's Encrypt から取得した HTTPS 証明書は自動的に更新されます。** 一度セットアップすれば、基本的にメンテナンスフリーで動作します。

```
● akebi-keyless-server.service - Akebi Keyless Server Service
     Loaded: loaded (/etc/systemd/system/akebi-keyless-server.service; enabled; vendor preset: enabled)
     Active: active (running) since Sat 2022-05-21 07:31:34 UTC; 2h 59min ago
TriggeredBy: ● akebi-keyless-server.socket
   Main PID: 767 (akebi-keyless-s)
      Tasks: 7 (limit: 1112)
     Memory: 7.8M
     CGroup: /system.slice/akebi-keyless-server.service
             └─767 /home/ubuntu/Akebi/akebi-keyless-server
```

`systemctl status akebi-keyless-server.service` がこのようになっていれば、正しく Keyless Server を起動できています。

```
$ sudo systemctl stop akebi-keyless-server.service
$ sudo systemctl stop akebi-keyless-server.socket
```

Keyless Server サービスを終了したい際は、以上のコマンドを実行してください。

### HTTPS Server のセットアップ

#### ビルド

HTTPS Server のビルドには、Go 1.18 と make がインストールされている環境が必要です。ここではすでにインストールされているものとして説明します。  

> [!NOTE]  
> Windows 版の make は [こちら](http://gnuwin32.sourceforge.net/packages/make.htm) からインストールできます。  
> 2006 年から更新されていませんが、Windows 10 でも普通に動作します。それだけ完成されたアプリケーションなのでしょう。

```bash
$ git clone git@github.com:tsukumijima/Akebi.git
$ cd Akebi

# 現在のプラットフォーム向けにビルド
$ make build-https-server

# すべてのプラットフォーム向けにビルド
# Windows (64bit), Linux (x64), Linux (arm64) 向けの実行ファイルを一度にクロスコンパイルする
$ make build-https-server-all-platforms
```

- Windows: `akebi-keyless-server.exe`
- Linux (x64): `akebi-keyless-server` (拡張子なし)
- Linux (arm64): `akebi-keyless-server-arm` (拡張子なし)

ビルドされた実行ファイルは、それぞれ Makefile と同じフォルダに出力されます。  
出力されるファイル名は上記の通りです。適宜リネームしても構いません。

#### HTTPS Server の設定

HTTPS Server は、設定を実行ファイルと同じフォルダにある `akebi-keyless-server.json` から読み込みます。Keyless Server 同様、JSONC (JSON with comments) で書かれています。  

設定はコマンドライン引数からも行えます。引数はそれぞれ設定ファイルの項目に対応しています。  
設定ファイルが配置されているときにコマンドライン引数を指定した場合は、コマンドライン引数の方の設定が優先されます。

- `listen_address`: HTTPS リバースプロキシをリッスンするアドレスを指定します。
  - コマンドライン引数では `--listen-address` に対応します。
  - 基本的には `0.0.0.0:(ポート番号)` のようにしておけば OK です。
- `proxy_pass_url`: リバースプロキシする HTTP サーバーの URL を指定します。
  - コマンドライン引数では `--proxy-pass-url` に対応します。
- `keyless_server_url`: Keyless Server の URL を指定します。 
  - コマンドライン引数では `--keyless-server-url` に対応します。
- `custom_certificate`: Keyless Server を使わず、カスタムの HTTPS 証明書/秘密鍵を使う場合に設定します。
  - コマンドライン引数では `--custom-certificate` `--custom-private-key` に対応します。
  - 普通に HTTPS でリッスンするのと変わりませんが、Keyless Server を使うときと HTTPS サーバーを共通化できること、HTTP/2 に対応できることがメリットです。

#### HTTPS リバースプロキシの起動

HTTPS Server は実行ファイル単体で動作します。  
`akebi-keyless-server.json` を実行ファイルと同じフォルダに配置しない場合は、実行時にコマンドライン引数を指定する必要があります。

```bash
$ ./akebi-https-server
2022/05/22 03:49:36 Info:  Starting HTTPS reverse proxy server...
2022/05/22 03:49:36 Info:  Listening on 0.0.0.0:3000, Proxing http://your-http-server-url:8080/.
```

**この状態で https://local.local.example.com:3000/ にアクセスしてプロキシ元のサイトが表示されれば、正しく HTTPS 化できています！！**

もちろん、たとえば PC のローカル IP が 192.168.1.11 なら、https://192-168-1-11.local.example.com:3000/ でもアクセスできるはずです。

HTTPS Server は Ctrl + C で終了できます。  
設定内容にエラーがあるときはログが表示されるので、それを確認してみてください。

> [!NOTE]  
> ドメインの本来 IP アドレスを入れる部分に **`my` / `local` / `localhost` と入れると、特別に 127.0.0.1（ループバックアドレス）に名前解決されるように設定しています。**  
`127-0-0-1.local.example.com` よりもわかりやすいと思います。ローカルで開発する際にお使いください。

**HTTPS Server は HTTP/2 に対応しています。** HTTP/2 は HTTPS でしか使えませんが、サイトを HTTPS 化することで、同時に HTTP/2 に対応できます。

> [!NOTE]  
> どちらかと言えば、Golang の標準 HTTP サーバー ([http.Server](https://pkg.go.dev/net/http#Server)) が何も設定しなくても HTTP/2 に標準対応していることによるものです。

カスタムの証明書/秘密鍵を指定できるのも、Keyless Server を使わずに各自用意した証明書で HTTPS 化するケースと実装を共通化できるのもありますが、**HTTPS Server を間に挟むだけでかんたんに HTTP/2 に対応できる**のが大きいです。

[Uvicorn](https://github.com/encode/uvicorn) など、HTTP/2 に対応していないアプリケーションサーバーはそれなりにあります。本来は NGINX などを挟むべきでしょうけど、一般ユーザーに配布するアプリケーションでは、簡易な HTTP サーバーにせざるを得ないことも多々あります。  
そうした場合でも、**アプリケーション本体の実装に手を加えることなく、アプリケーション本体の起動と同時に HTTPS Server を起動するだけで、HTTPS 化と HTTP/2 対応を同時に行えます。**

```bash
$ ./akebi-https-server --listen-address 0.0.0.0:8080 --proxy-pass-url http://192.168.1.11:8000
2022/05/22 03:56:50 Info:  Starting HTTPS reverse proxy server...
2022/05/22 03:56:50 Info:  Listening on 0.0.0.0:8080, Proxing http://192.168.1.11:8000.
```

`--listen-address` や `--proxy-pass-url` オプションを指定して、リッスンポートやプロキシ対象の HTTP サーバーの URL を上書きできます。

```bash
$ ./akebi-https-server -h
Usage of C:\Develop\Akebi\akebi-https-server.exe:
  -custom-certificate string
        Optional: Use your own HTTPS certificate instead of Akebi Keyless Server.
  -custom-private-key string
        Optional: Use your own HTTPS private key instead of Akebi Keyless Server.
  -keyless-server-url string
        URL of HTTP server to reverse proxy.
  -listen-address string
        Address that HTTPS server listens on.
        Specify 0.0.0.0:port to listen on all interfaces.
  -mtls-client-certificate string
        Optional: Client certificate of mTLS for akebi.example.com (Keyless API).
  -mtls-client-certificate-key string
        Optional: Client private key of mTLS for akebi.example.com (Keyless API).
  -proxy-pass-url string
        URL of HTTP server to reverse proxy.
```

`-h` オプションでヘルプが表示されます。

## 技術解説と注意

### Keyless の仕組み

![](https://blog.cloudflare.com/content/images/2014/Sep/cloudflare_keyless_ssl_handshake_diffie_hellman.jpg)

**秘密鍵をユーザーに公開せずに正規の HTTPS サーバーを立てられる**というトリックには（”Keyless” の由来）、Cloudflare の [Keyless SSL](https://blog.cloudflare.com/announcing-keyless-ssl-all-the-benefits-of-cloudflare-without-having-to-turn-over-your-private-ssl-keys/) と同様の手法が用いられています。

サイトを Cloudflare にキャッシュさせる場合、通常は Cloudflare 発行の証明書を利用できます。一方、企業によっては、EV 証明書を使いたいなどの理由でカスタム証明書を使うケースがあるようです。  
Cloudflare の仕組み上、カスタム証明書を利用する際は、その証明書と秘密鍵を Cloudflare に預ける必要があります。Keyless SSL は、Cloudflare でカスタム証明書を使いたいが、コンプライアンス上の理由でカスタム証明書の秘密鍵を社外に預けられない企業に向けたサービスです。  

Keyless SSL では、秘密鍵を社外に出せない企業側が「Key Server」をホストします。Key Server は、**TLS ハンドシェイクのフローのうち、秘密鍵を必要とする処理を Cloudflare の Web サーバーに代わって行う** API サーバーです。

具体的には、鍵交換アルゴリズムが RSA 法のときは、（ブラウザから送られてきた）公開鍵で暗号化された Premaster Secret を秘密鍵で復号し、それを Cloudflare のサーバーに返します。  
鍵交換アルゴリズムが DHE (Diffie-Hellman) 法のときはもう少し複雑で、Client Random・Server Random・Server DH Parameter をハッシュ化したものに秘密鍵でデジタル署名を行い、それを Cloudflare のサーバーに返します。  
複雑で難解なこともあり私も正しく説明できているか自信がないので、詳細は [公式の解説記事](https://blog.cloudflare.com/keyless-ssl-the-nitty-gritty-technical-details/) に譲ります…。

-----

この Keyless SSL の **「秘密鍵がなくても、証明書と Key Server さえあれば HTTPS 化できる」** という特徴を、同じく秘密鍵を公開できない今回のユースケースに適用したものが、ncruces 氏が開発された [keyless](https://github.com/ncruces/keyless) です。

> [!NOTE]  
> 前述しましたが、Akebi Keyless Server は keyless のサーバー部分のコードのフォークです。

**Keyless SSL の「Key Server」に相当するものが、Keyless Server がリッスンしている API サーバーです。**（以下、Keyless API と呼称）  
`/certificate` エンドポイントは、Keyless Server が保管しているワイルドカード証明書をそのまま返します。  
`/sign` エンドポイントは、HTTPS Server からワイルドカード証明書の SHA-256 ハッシュとClient Random・Server Random・Server DH Parameter のハッシュを送り、送られた証明書のハッシュに紐づく秘密鍵で署名された、デジタル署名を返します。

keyless の作者の [ncruces 氏によれば](https://github.com/cunnie/sslip.io/issues/6#issuecomment-778914231)、Keyless SSL と異なり、「問題を単純化するため」鍵交換アルゴリズムは DHE 法 (ECDHE)、公開鍵/秘密鍵は ECDSA 鍵のみに対応しているとのこと。  
Keyless Server のセットアップで生成された秘密鍵のサイズが小さいのはそのためです（ECDSA は RSA よりも鍵長が短い特徴があります）。

> [!NOTE]  
> 図だけを見れば RSA 鍵交換アルゴリズムの方が単純に見えますが、ECDHE with ECDSA の方が新しく安全で速いそうなので、それを加味して選定したのかもしれません。

Keyless SSL とは手法こそ同様ですが、**Key Server との通信プロトコルは異なるため（keyless では大幅に簡略化されている）、Keyless SSL と互換性があるわけではありません。**

### 中間者攻撃のリスクと mTLS (TLS相互認証)

この手法は非常に優れていますが、**中間者攻撃 (MitM) のリスクは残ります。**  
証明書と秘密鍵がそのまま公開されている状態と比較すれば、攻撃の難易度は高くなるでしょう。とはいえ、Keyless API にはどこからでもアクセスできるため、やろうと思えば中間者攻撃できてしまうかもしれません（セキュリティエンジニアではないので詳しいことはわからない…）。

そこで、ncruces 氏は Keyless API を [mTLS (TLS相互認証)](https://e-words.jp/w/mTLS.html) で保護し、**正しいクライアント証明書/秘密鍵を持っている Keyless API クライアントのみ Keyless API にアクセスできるようにする**ことを提案しています。

正しいクライアント証明書/秘密鍵がなければ Keyless API にアクセスできないため、中間者攻撃のリスクを減らせます。  
とはいえ、**クライアント証明書/秘密鍵が盗まれてしまっては意味がありません。** ncruces 氏自身も[「最終的には、難読化や DRM のような方法になります」](https://github.com/cunnie/sslip.io/issues/6#issuecomment-778914231)とコメントしています。

なお、私のユースケースでは **『ローカル LAN 上のサイトをブラウザに形式上 HTTPS と認識させられれば正直中間者攻撃のリスクはどうでもいい』** というものだったため、mTLS は利用していません。

> だいたい、もし通信内容を中間者攻撃されるようなローカル LAN があるのなら、そのネットワークはいろいろな意味で終わってると思う…。

…とは言ったものの、一応 Akebi でも mTLS に対応しています。正確には keyless で対応されていたので HTTPS Server でも使えるようにした程度のものですが…。

```bash
openssl req -newkey rsa:2048 -nodes -x509 -days 365 -out client_ca_cert.pem -keyout client_ca_private_key.pem
openssl genrsa -out client_private_key.pem 2048
openssl req -new -key client_private_key.pem -days 365 -out client_cert.csr
openssl x509 -req -in client_cert.csr -CA client_ca_cert.pem -CAkey client_ca_private_key.pem -out client_cert.pem -days 365 -sha256 -CAcreateserial
rm client_ca_cert.srl
rm client_cert.csr
```

mTLS のクライアントCA証明書とクライアント証明書を作成するには、上記のコマンドを実行します。

`client_ca_cert.pem`・`client_ca_private_key.pem` がクライアント CA 証明書/秘密鍵、`client_cert.pem`・`client_private_key.pem` がクライアント証明書/秘密鍵です。

Keyless Server の設定では、`keyless_api.client_ca` に mTLS のクライアント CA 証明書 (`client_ca_cert.pem`) へのパスを指定します。  
設定の反映には Keyless Server サービスの再起動が必要です。

HTTPS Server の設定では、`mtls.client_certificate`・`mtls.client_certificate_key` に mTLS のクライアント証明書/秘密鍵 (`client_cert.pem`・`client_private_key.pem`) へのパスを指定します。

**この状態で HTTPS Server がリッスンしているサイトにアクセスできれば、mTLS を有効化できています。**  
Keyless Server にクライアント CA 証明書を設定したまま HTTPS Server の mTLS 周りの設定を外すと、Keyless API にアクセスできなくなっているはずです。

### HTTPS リバースプロキシというアプローチ

Akebi では、Keyless Server を使い HTTPS 化するためのアプローチとして、HTTPS サーバーを背後の HTTP サーバーのリバースプロキシとして立てる、という方法を採用しています。

一方、フォーク元の keyless は、Golang で書かれた Web サーバーの TLS 設定に、Keyless のクライアントライブラリの関数 ([GetCertificate()](https://github.com/ncruces/keyless/blob/main/keyless.go#L21)) をセットすることで、「直接」HTTPS 化するユースケースを想定して書かれています。

このアプローチは、確かにアプリケーションサーバーが Golang で書かれているケースではぴったりな一方で、**アプリケーションサーバーが Golang 以外の言語で書かれている場合は使えません。**  
とはいえ、他の言語で書かれたアプリケーションサーバーを、HTTPS 化するためだけに Golang で書き直すのは非現実的です。それぞれの言語の利点もありますし。

-----

そうなると、一見 keyless のクライアントライブラリを Python や Node.js など、ほかの言語に移植すれば良いように見えます。ところが、**ほとんどの言語において、ライブラリの移植は不可能なことがわかりました。**

実際に keyless クライアントに相当する実装を Python に移植できないか試したのですが、実は **Python は TLS 周りの実装を OpenSSL に丸投げしています。** 標準モジュールの `ssl` も、その実態は OpenSSL のネイティブライブラリのラッパーにすぎません。  
さらに、`ssl` モジュールでは、**TLS ハンドシェイクを行う処理が [`SSLContext.do_handshake()`](https://docs.python.org/ja/3.10/library/ssl.html#ssl.SSLSocket.do_handshake) の中に隠蔽されているため、TLS ハンドシェイクの内部処理に介入できないことが分かりました。**  
Golang では TLS ハンドシェイクの細かい設定を行う [struct](https://pkg.go.dev/crypto/tls#Config) が用意されていますが、Python ではそれに相当する API を見つけられませんでした。おそらくないんだと思います…。

Node.js の [TLS](https://nodejs.org/api/tls.htm) ライブラリも軽く調べてみましたが、Python と比べると API もきれいでより低レベルなカスタマイズができるものの、TLS ハンドシェイクそのものに介入するための API は見つけられませんでした。  
複雑で難解な上にフローが決まりきっている TLS ハンドシェイクの内部処理にわざわざ割り込むユースケースが（こうした特殊なケースを除いて）ほぼ皆無なことは火を見るより明らかですし、仕方ないとは思います。

> TLS 周りの実装は下手すれば脆弱性になりかねませんし、専門知識のない一般のプログラマーがいじれるとかえってセキュリティリスクが高まる、という考えからなのかもしれません（実際そうだとは思います）。

見つけられていないだけで、keyless クライアントライブラリを移植可能な（TLS ハンドシェイクの深い部分まで介入できる）言語もあるかもしれません。ですが、すでに API の仕様上移植できない言語があるとなっては、直接 Keyless Server を使って HTTPS 化するアプローチは取りづらいです。

また、一般的な Web サービスではアプリケーションサーバーとインターネットとの間に Apache や NGINX などの Web サーバーを挟むことが多いですが、Apache や NGINX が keyless クライアントに対応していないことは言うまでもありません。Apache や NGINX のソースコードをいじればなんとかなるかもですが、そこまでするかと言われると…。

-----

そこで **「直接 keyless クライアントにできないなら、keyless に対応したリバースプロキシを作ればいいのでは？」と逆転の発想で編み出したのが、HTTPS リバースプロキシというアプローチです。**  

この方法であれば、**Keyless で HTTPS 化したい HTTP サーバーがどんな言語や Web サーバーを使っていようと関係なく、かんたんに HTTPS サーバーを立ち上げられます。**

リバースプロキシをアプリケーションサーバーとは別で起動させないといけない面倒さこそありますが、一度起動してしまえば、明示的に終了するまでリッスンしてくれます。アプリケーションサーバーの起動時に同時に起動し、終了時に同時に終了させるようにしておくと良いでしょう。

また、HTTPS Server は単一バイナリだけで動作します。引数を指定すれば設定ファイル (`akebi-https-server.json`) がなくても起動できますし、設定ファイルを含めても、必要なのは2ファイルだけです。  
Apache や NGINX を一般的な PC に配布するアプリケーションに組み込むのはいささか無理がありますが、これなら配布するアプリケーションにも比較的組み込みやすいのではないでしょうか。

### URL 変更について

HTTPS 化にあたっては、**今までの `http://192.168.1.11:3000/` のような IP アドレス直打ちの URL が使えなくなり、代わりに `https://192-168-1-11.local.example.com:3000/` のような URL でアクセスする必要がある点を、ユーザーに十分に周知させる必要があります。**  

> [!NOTE]  
> 一応 `https://192.168.1.11:3000/` でも使えなくはないですが、言うまでもなく証明書エラーが表示されます。

プライベート IP アドレスや mDNS のようなローカル LAN だけで有効なドメイン (例: `my-computer.local`) には正規の HTTPS 証明書を発行できないため、<u>**プライベート Web サイトで本物の HTTPS 証明書を使うには、いずれにせよインターネット上で有効なドメインにせざるを得ません。**</u>

そのため、オレオレ証明書を使わずに HTTPS 化したいのであれば、この変更は避けられません。  
ただ、**この URL 変更は十分に破壊的な変更になりえます。** 特にユーザーの多いプロダクトであれば、慎重に進めるべきでしょう。  
もしこの破壊的な変更を受け入れられないプロダクトであれば、HTTP でのアクセスを並行してサポートするか、正規の HTTPS 証明書を使うのを諦めるほかありません。

> [!NOTE]  
> HTTP・HTTPS を両方サポートできる（HTTP アクセスでは HTTPS を必要とする機能を無効化する）リソースがあるのなら、並行して HTTP アクセスをサポートするのもありです。

私のユースケースでは、HTTPS 化によって得られるメリットが URL 変更のデメリットを上回ると判断して、Akebi の採用を決めました。メリットとデメリットを天秤にかけて、採用するかどうかを考えてみてください。  
**HTTPS が必要な機能をさほど使っていない/使う予定がないのであれば、ずっと HTTP のまま（現状維持）というのも全然ありだと思います。**

-----

また、逸般の誤家庭で使われがちなプロダクトでは、**『自分が所有しているドメインと証明書を使いたい』『開発者側が用意したドメインが気に入らない』『オレオレ証明書でいいから IP アドレス直打ちでアクセスさせろ』** といった声が上がることも想定されます。

そうした要望に応えるのなら、必然的にカスタムの HTTPS 証明書/秘密鍵を使って HTTPS サーバーを起動することになります。  
ただ、一般ユーザー向けには Akebi の HTTPS リバースプロキシを挟み、カスタム証明書を使いたい逸般ユーザー向けには直接アプリケーション側で HTTPS サーバーをリッスンし… と分けていては、実装が煩雑になることは目に見えています。

そこで、**HTTPS Server 自体に、カスタムの証明書/秘密鍵を使って HTTPS リバースプロキシをリッスンできる設定とコマンドライン引数を用意しました。**  
この機能を使うことで、HTTPS サーバーの役目を Akebi HTTPS Server に一元化できます。

詳しくは [HTTPS Server の設定](#https-server-の設定) で説明していますが、HTTPS Server では、**設定ファイルに記載の設定よりも、コマンドライン引数に指定した設定の方が優先されます。**  
これを利用して、HTTPS Server の起動コマンドに、アプリケーション側の設定でカスタムの証明書/秘密鍵が指定されたときだけ `--custom-certificate` / `--custom-private-key` を指定すれば、**設定ファイルを書き換えることなく、カスタム証明書を使って HTTPS Server を起動できます。**  
HTTPS サーバーを別途用意するよりも、はるかにシンプルな実装になるはずです。

また、カスタム証明書での HTTPS 化を HTTPS Server で行うことで、前述したように HTTP/2 にも対応できます。  
HTTP/2 対応によって爆速になる、ということはあまりないとは思いますが、多かれ少なかれパフォーマンスは向上するはずです。

-----

カスタム証明書/秘密鍵を使いたい具体的なユースケースとして、**[Tailscale の HTTPS 有効化機能](https://tailscale.com/kb/1153/enabling-https/) を利用するケースが考えられます。**

> [!NOTE]  
> Tailscale は、P2P 型のメッシュ VPN をかんたんに構築できるサービスです。
> Tailscale に接続していれば、どこからでもほかの Tailscale に接続されているデバイスにアクセスできます。

`tailscale cert` コマンドを実行すると、`[machine-name].[domain-alias].ts.net` のフォーマットのドメインで利用できる、HTTPS 証明書と秘密鍵が発行されます。  
この証明書は、ホスト名が `[machine-name].[domain-alias].ts.net` であれば同じ PC 内のどんなプライベート Web サイトでも使える、Let's Encrypt 発行の正規の証明書です。

**Tailscale から発行されたカスタムの証明書/秘密鍵を HTTPS Server に設定すると、`https://[machine-name].[domain-alias].ts.net:3000/` の URL でアプリケーションに HTTPS でアクセスできるようになります。**  
Keyless Server を利用する機能が無効化されるため、`https://192-168-1-11.local.example.com:3000/` の URL でアクセスできなくなる点はトレードオフです。

 Tailscale を常に経由してプライベート Web サイトにアクセスするユーザーにとっては、IP アドレスそのままよりもわかりやすい URL でアクセスできるため、Keyless Server よりも良い選択肢かもしれません。

## License

[MIT License](License.txt)
