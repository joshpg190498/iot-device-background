# Despliegue de fedsbc en un SBC

Instala en un dispositivo (Raspberry Pi, aarch64) los tres componentes de la
plataforma y los deja corriendo como servicios del sistema.

| Servicio | Componente | Lenguaje |
|---|---|---|
| `fedsbc-agent` | Agente de telemetría (CPU, PSI, frecuencia real, temperatura, RAM) | Go |
| `fedsbc-model-manager` | Agente de modelos: recibe, verifica (sha256) y activa versiones | Go |
| `fedsbc-supernode` | SuperNode de Flower: entrenamiento federado | Python |

## Requisitos del dispositivo

- Raspberry Pi OS **64 bits** (aarch64), con acceso a internet.
- Go y git: `sudo apt install -y golang git`
- `vcgencmd` para leer la frecuencia real del firmware: `sudo apt install -y raspi-utils-core`
  (en Raspberry Pi OS suele venir instalado).
- PSI activo en el kernel: si `ls /proc/pressure` falla, agregar `psi=1` al final de la
  línea de `/boot/firmware/cmdline.txt` y reiniciar.
- El dispositivo registrado en el servidor (web app), que crea sus lecturas en
  `DEVICE_READING_SETTINGS`.

## Instalación

```bash
git clone https://github.com/joshpg190498/iot-device-background
cd iot-device-background
cp .env.example .env
nano .env                      # completar los valores vacíos
sudo ./deploy/install.sh
```

Valores a completar en `.env`:

| Variable | Ejemplo | Qué define |
|---|---|---|
| `ID_DEVICE` | `NODE002` | Identidad del dispositivo (telemetría, modelos y aprendizaje federado) |
| `MQTT_HOST` | `192.168.1.41` | Broker MQTT del servidor |
| `SERVER_HTTP_BASE` | `http://192.168.1.41:8000` | Servidor de archivos de modelos |
| `SUPERLINK` | `192.168.1.41:9192` | SuperLink de Flower |
| `PARTITION_ID` | `1` | Partición de datos de este nodo en los experimentos (`0 .. N-1`) |
| `NUM_PARTITIONS` | `2` | Cantidad total de particiones (= nodos del experimento) |

Los certificados MQTT (`ca-crt.pem`, `client-crt.pem`, `client-key.pem`) se toman de
la carpeta `certs/` del repo.

El instalador primero verifica todo (`.env` completo, partición válida, certificados,
Go). Si falta algo, se detiene **sin modificar nada** e indica qué falta. Después:

1. Compila los agentes Go. Si falla, se detiene sin tocar lo que está corriendo.
2. Crea el usuario de sistema `fedsbc` y la estructura en `/opt/fedsbc`.
3. Copia `.env` a `/opt/fedsbc/etc/fedsbc.env` (con las rutas de `/opt/fedsbc`) y los
   certificados.
4. Crea el entorno de Flower con las versiones fijadas en `flower/requirements.lock`.
5. Instala y arranca los servicios, que también arrancan solos al encender el equipo.

En la primera instalación, cerrar y volver a abrir la sesión SSH para poder entrar a
`/opt/fedsbc` sin sudo.

## Cambiar la configuración o actualizar

El `.env` del repo es la única fuente de configuración: el instalador lo copia en
cada ejecución.

```bash
nano .env && sudo ./deploy/install.sh        # cambiar un valor
git pull && sudo ./deploy/install.sh         # actualizar el código
```

Se puede ejecutar las veces que haga falta: conserva los modelos desplegados, la base
local del agente y el entorno de Python (solo lo reconstruye si cambió
`requirements.lock`).

## Estructura instalada

```
/opt/fedsbc/
├── bin/                data-acquisition, model-manager (.env -> ../etc/fedsbc.env)
├── etc/                fedsbc.env (copia del .env del repo)
├── certs/              certificados MQTT
├── frameworks/flower/  .venv, start-supernode.sh, home/ (apps recibidas por Flower)
├── models/<modelo>/    versions/N/model.tflite, current -> versions/N
└── var/                agent.sqlite, keras/ (datasets)
```

## Permisos

| Quién | Acceso |
|---|---|
| Servicios (usuario de sistema `fedsbc`, sin login) | Leen `etc/` y `certs/`; escriben solo en `models/`, `var/` y `frameworks/flower/home/` |
| Operador (el usuario que ejecutó el instalador) | Lee y edita todo `/opt/fedsbc` sin sudo (grupo `fedsbc`) |
| Otros usuarios | Sin acceso al contenido |

## Operación

```bash
systemctl is-active fedsbc-agent fedsbc-model-manager fedsbc-supernode
journalctl -u fedsbc-supernode -f          # logs en vivo (idem para los otros)
sudo systemctl restart fedsbc-agent
```

## Desinstalar

```bash
sudo systemctl disable --now fedsbc-agent fedsbc-model-manager fedsbc-supernode
sudo rm /etc/systemd/system/fedsbc-*.service && sudo systemctl daemon-reload
sudo rm -rf /opt/fedsbc && sudo userdel fedsbc
```
