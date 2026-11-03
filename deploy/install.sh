#!/usr/bin/env bash
# =============================================================================
# fedsbc - device (SBC) installer
#
#   cp .env.example .env        # fill in ID_DEVICE, server and partition
#   sudo ./deploy/install.sh
#
# Installs everything in /opt/fedsbc and starts it as systemd services:
#   fedsbc-agent          telemetry agent (Go, high priority)
#   fedsbc-model-manager  model agent (Go)
#   fedsbc-supernode      Flower SuperNode (Python)
#
# The .env in the repo root is the single source of config. The installer copies
# it to /opt/fedsbc/etc/fedsbc.env on every run. To change a value, edit .env and
# run the installer again. Safe to run many times: deployed models, the agent
# database and the Python environment are kept.
# =============================================================================
set -euo pipefail

PREFIX="${FEDSBC_PREFIX:-/opt/fedsbc}"
SVC_USER="fedsbc"
UNITS=(fedsbc-agent fedsbc-model-manager fedsbc-supernode)
PYTHON_VERSION="3.11"
REQUIRED=(ID_DEVICE MQTT_PROTOCOL MQTT_HOST MQTT_PORT SERVER_HTTP_BASE SUPERLINK PARTITION_ID NUM_PARTITIONS)

step() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
ok()   { printf '    \033[32mok\033[0m  %s\n' "$*"; }
warn() { printf '    \033[33m!!\033[0m  %s\n' "$*"; }
die()  { printf '\n\033[31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

# Read KEY from an env file (value without quotes, spaces or trailing comment)
get_kv() { sed -n "s/^$2=\([^ #]*\).*/\1/p" "$1" | tr -d '\r"' | head -1; }
# Set KEY=VALUE in an env file (replace the line, or add it if missing)
set_kv() {
  if grep -q "^$2=" "$1"; then sed -i "s#^$2=.*#$2=$3#" "$1"; else echo "$2=$3" >> "$1"; fi
}

# ------------------------------------------------------------ 0. checks
# Everything is checked before touching the system.
[ "$(id -u)" -eq 0 ] || die "ejecutar con sudo:  sudo ./deploy/install.sh"
OPERATOR="${SUDO_USER:-}"
[ -n "$OPERATOR" ] && [ "$OPERATOR" != root ] || die "ejecutar con sudo desde su usuario (no como root directamente)"
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEPLOY="$REPO/deploy"
SRC_ENV="$REPO/.env"
SRC_CERTS="$REPO/certs"
as_op() { sudo -u "$OPERATOR" -H bash -lc "$*"; }

echo "fedsbc -> $PREFIX   (repo: $REPO, operador: $OPERATOR)"
step "Verificando requisitos"

[ -f "$SRC_ENV" ] || die "no existe $SRC_ENV
       Crearlo desde la plantilla y completarlo:
         cp .env.example .env && nano .env"
missing=()
for k in "${REQUIRED[@]}"; do [ -n "$(get_kv "$SRC_ENV" "$k")" ] || missing+=("$k"); done
[ ${#missing[@]} -eq 0 ] || die "faltan valores en .env: ${missing[*]}"
PART="$(get_kv "$SRC_ENV" PARTITION_ID)"; NPART="$(get_kv "$SRC_ENV" NUM_PARTITIONS)"
[[ "$PART" =~ ^[0-9]+$ && "$NPART" =~ ^[0-9]+$ ]] && [ "$PART" -lt "$NPART" ] \
  || die "PARTITION_ID debe ser un número entre 0 y NUM_PARTITIONS-1 (ahora: $PART de $NPART)"
ok ".env completo (ID_DEVICE=$(get_kv "$SRC_ENV" ID_DEVICE), partición $PART de $NPART)"

if ls "$SRC_CERTS"/*.pem >/dev/null 2>&1; then
  ok "certificados en $SRC_CERTS"
elif ls "$PREFIX/certs"/*.pem >/dev/null 2>&1; then
  ok "certificados ya instalados en $PREFIX/certs"
else
  die "no hay certificados MQTT: copiar ca-crt.pem, client-crt.pem y client-key.pem a $SRC_CERTS"
fi

GO_BIN="$(as_op 'command -v go' 2>/dev/null || true)"
if [ -z "$GO_BIN" ] && [ -x /usr/local/go/bin/go ]; then GO_BIN=/usr/local/go/bin/go; fi
[ -n "$GO_BIN" ] || die "no se encontró Go:  sudo apt install -y golang"
ok "Go: $(as_op "'$GO_BIN' version" | cut -d' ' -f3)"

[ -d /proc/pressure ] || warn "PSI no está activo: agregar psi=1 a /boot/firmware/cmdline.txt y reiniciar (la telemetría de presión quedará vacía)"
command -v vcgencmd >/dev/null || warn "vcgencmd no está instalado (sudo apt install -y raspi-utils-core): la frecuencia se leerá del kernel"

# ------------------------------------------------------- 1. Go binaries
# Build into a temp folder before stopping anything: if the build fails,
# nothing is changed and the services keep running the previous version.
step "Compilando agentes Go"
BUILD_DIR="$(as_op 'mktemp -d')"
if ! as_op "cd '$REPO' && '$GO_BIN' build -ldflags '-s -w' -o '$BUILD_DIR/data-acquisition' ./modules/data-acquisition && '$GO_BIN' build -ldflags '-s -w' -o '$BUILD_DIR/model-manager' ./modules/model-manager"; then
  rm -rf "$BUILD_DIR"
  die "falló la compilación (ver el error de Go arriba). No se modificó nada."
fi
ok "compilados data-acquisition y model-manager"

# ------------------------------------------- 2. service user and folders
step "Usuario de sistema y directorios"
if ! id "$SVC_USER" >/dev/null 2>&1; then
  useradd --system --home-dir "$PREFIX" --no-create-home --shell /usr/sbin/nologin "$SVC_USER"
  ok "usuario $SVC_USER creado"
else
  ok "usuario $SVC_USER ya existe"
fi
# video group: needed to run vcgencmd (real CPU frequency from the firmware)
if getent group video >/dev/null; then usermod -aG video "$SVC_USER"; fi
install -d "$PREFIX"/{bin,etc,certs,models,var/keras,frameworks/flower/home}
ok "estructura en $PREFIX"

# ------------------------------------------------- 3. stop and install
step "Instalando"
systemctl stop "${UNITS[@]}" 2>/dev/null || true
install -m 755 "$BUILD_DIR/data-acquisition" "$BUILD_DIR/model-manager" "$PREFIX/bin/"
rm -rf "$BUILD_DIR"
ok "binarios en $PREFIX/bin"

# Config: copy of the repo .env, with Unix line endings and the /opt paths
ENV_FILE="$PREFIX/etc/fedsbc.env"
tr -d '\r' < "$SRC_ENV" > "$ENV_FILE"
set_kv "$ENV_FILE" CERTS_DIR "$PREFIX/certs"
set_kv "$ENV_FILE" MODELS_DIR "$PREFIX/models"
set_kv "$ENV_FILE" SQLITE_DB_PATH "$PREFIX/var/agent.sqlite"
# The Go agents read .env next to the binary: link it to the single config file
ln -sfn "$ENV_FILE" "$PREFIX/bin/.env"
ok "configuración: .env -> $ENV_FILE"

if ls "$SRC_CERTS"/*.pem >/dev/null 2>&1; then
  cp "$SRC_CERTS"/*.pem "$PREFIX/certs/"
  ok "certificados copiados a $PREFIX/certs"
fi

install -m 755 "$DEPLOY/flower/start-supernode.sh" "$PREFIX/frameworks/flower/start-supernode.sh"

# ------------------------------------------------- 4. Flower environment
step "Entorno de Flower (Python $PYTHON_VERSION, versiones fijadas)"
FW="$PREFIX/frameworks/flower"
LOCK="$DEPLOY/flower/requirements.lock"
STAMP="$FW/.venv/.fedsbc-lock.sha256"
LOCK_SUM="$(sha256sum "$LOCK" | cut -d' ' -f1)"
if [ -x "$FW/.venv/bin/flower-supernode" ] && [ "$(cat "$STAMP" 2>/dev/null)" = "$LOCK_SUM" ]; then
  ok "entorno al día (sin cambios en requirements.lock)"
else
  # uv installs the exact Python version that TensorFlow needs
  UV="$(as_op 'command -v uv' 2>/dev/null || true)"
  if [ -z "$UV" ]; then
    [ -x "$FW/uv/uv" ] || curl -LsSf https://astral.sh/uv/install.sh | env UV_INSTALL_DIR="$FW/uv" UV_NO_MODIFY_PATH=1 sh >/dev/null
    UV="$FW/uv/uv"
  fi
  echo "    creando entorno (puede tardar varios minutos: TensorFlow)..."
  export UV_PYTHON_INSTALL_DIR="$FW/python" UV_CACHE_DIR="$FW/.uv-cache"
  rm -rf "$FW/.venv"
  "$UV" venv --python "$PYTHON_VERSION" "$FW/.venv" >/dev/null
  "$UV" pip install --python "$FW/.venv/bin/python" -r "$LOCK" >/dev/null
  echo "$LOCK_SUM" > "$STAMP"
  ok "entorno creado en $FW/.venv"
fi

# ---------------------------------------------------------- 5. permissions
# Permission model:
#   - services run as $SVC_USER (no login);
#   - the operator (the user who ran the installer) joins group $SVC_USER and can
#     read and edit everything without sudo; root (sudo) can always do it;
#   - etc/ and certs/: owned by the operator; services can only read them;
#   - models/, var/, frameworks/flower/home: owned by the service, group can write;
#   - setgid on folders: new files keep the group. UMask=0007 in the units makes
#     files created by the services editable by the group;
#   - other users: no access.
step "Permisos"
usermod -aG "$SVC_USER" "$OPERATOR"
chown -R "$OPERATOR:$SVC_USER" "$PREFIX"
chown -R "$SVC_USER:$SVC_USER" "$PREFIX/models" "$PREFIX/var" "$FW/home"
chown "$OPERATOR:$SVC_USER" "$PREFIX/models" "$PREFIX/var"
chmod -R g+rwX,o-rwx "$PREFIX"
find "$PREFIX" -type d -exec chmod g+s {} +
chmod -R g-w "$PREFIX/etc" "$PREFIX/certs"
chmod 755 "$PREFIX"   # allow path traversal; content is still protected
ok "$OPERATOR (grupo $SVC_USER): lectura/escritura en $PREFIX  |  servicios: solo lectura en etc/ y certs/"

# ---------------------------------------------------------- 6. services
step "Servicios systemd"
for u in "${UNITS[@]}"; do
  sed "s#/opt/fedsbc#$PREFIX#g" "$DEPLOY/systemd/$u.service" > "/etc/systemd/system/$u.service"
done
systemctl daemon-reload
systemctl enable "${UNITS[@]}" >/dev/null 2>&1
systemctl restart "${UNITS[@]}"
sleep 3

# ------------------------------------------------------------- summary
step "Resultado"
fail=0
for u in "${UNITS[@]}"; do
  st="$(systemctl is-active "$u" || true)"
  if [ "$st" = active ]; then ok "$(printf '%-22s' "$u") active"; else warn "$(printf '%-22s' "$u") $st"; fail=1; fi
done
echo
echo "  Configuración:   editar .env del repo y volver a ejecutar  sudo ./deploy/install.sh"
echo "  Logs:            journalctl -u fedsbc-agent -f   (o fedsbc-model-manager / fedsbc-supernode)"
[ "$fail" -eq 0 ] || die "algún servicio no quedó activo: ver  journalctl -u <servicio> -n 30 --no-pager"
echo
echo "fedsbc instalado. Primera instalación: cierre y vuelva a abrir la sesión SSH"
echo "para que $OPERATOR pueda entrar a $PREFIX sin sudo."
