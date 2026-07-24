// Neutral (non-regional) Spanish translations, keyed by the English source
// string. Any key absent here falls back to its English text, so partial
// coverage never breaks the UI. Technical tokens (NAT, IP, CPU, vCPU, TabVM,
// VBoxManage, Guest Additions, guestcontrol) are intentionally left untranslated.
export const es: Record<string, string> = {
  // ----- AppShell / navigation -----
  Machines: 'Máquinas',
  Activity: 'Actividad',
  Agent: 'Agente',
  Workspace: 'Espacio de trabajo',
  System: 'Sistema',
  Help: 'Ayuda',
  Docs: 'Docs',
  docs: 'documentación',
  workspace: 'espacio',
  machines: 'máquinas',
  'virtual machines': 'máquinas virtuales',
  activity: 'actividad',
  agent: 'agente',
  'agent offline': 'agente desconectado',
  'agent online': 'agente conectado',
  'bound · 127.0.0.1': 'enlazado · 127.0.0.1',
  // ----- VM states (normalized backend vocabulary) -----
  running: 'en marcha',
  booting: 'iniciando',
  resuming: 'reanudando',
  paused: 'en pausa',
  saving: 'guardando',
  saved: 'guardada',
  stopping: 'deteniendo',
  'powered off': 'apagada',
  aborted: 'abortada',
  stuck: 'bloqueada',
  'Toggle sidebar': 'Alternar barra lateral',
  'Collapse sidebar': 'Contraer barra lateral',
  'Toggle theme': 'Cambiar tema',
  'Switch language': 'Cambiar idioma',
  'Accent color': 'Color de acento',
  Teal: 'Verde azulado',
  Pink: 'Rosa',
  Orange: 'Naranja',
  Yellow: 'Amarillo',
  Purple: 'Morado',
  Blue: 'Azul',

  // ----- MachinesView -----
  'Local VirtualBox machines, controlled from the browser like tabs.':
    'Máquinas VirtualBox locales, controladas desde el navegador como pestañas.',
  schema: 'esquema',
  'local state': 'estado local',
  uptime: 'tiempo activo',
  'Virtual machines': 'Máquinas virtuales',
  Refresh: 'Actualizar',
  'Discovering VirtualBox…': 'Detectando VirtualBox…',
  'No virtual machines found.': 'No se encontraron máquinas virtuales.',
  'Install Guest Additions': 'Instalar Guest Additions',
  'inserting…': 'insertando…',
  'disc inserted · run installer in VM': 'disco insertado · ejecute el instalador en la VM',
  'Update Guest Additions': 'Actualizar Guest Additions',
  'stopping…': 'deteniendo…',
  stop: 'detener',
  suspend: 'suspender',
  'suspending…': 'suspendiendo…',
  'Suspend {vm}': 'Suspender {vm}',
  'save state · resume later': 'guardar estado · reanudar luego',
  'force power off': 'forzar apagado',
  'powering off…': 'apagando…',
  'Force power off {vm}': 'Forzar el apagado de {vm}',
  'Force power off "{name}"? This is like pulling the power plug: the guest will not shut down cleanly and unsaved data inside it will be lost.':
    '¿Forzar el apagado de "{name}"? Es como desconectar el cable de alimentación: el sistema invitado no se apagará de forma correcta y los datos no guardados que contenga se perderán.',
  reset: 'reiniciar',
  delete: 'eliminar',
  'deleting…': 'eliminando…',
  'Delete VM': 'Eliminar VM',
  'Delete {vm}': 'Eliminar {vm}',
  'Delete "{name}" permanently? Its disks and configuration files will be removed from this computer. This cannot be undone.':
    '¿Eliminar "{name}" de forma permanente? Sus discos y archivos de configuración se eliminarán de este equipo. Esta acción no se puede deshacer.',
  'starting…': 'iniciando…',
  start: 'iniciar',
  'new tab': 'nueva pestaña',
  'open console': 'abrir consola',
  'Install Guest Additions on {vm}': 'Instalar Guest Additions en {vm}',
  'Update Guest Additions on {vm}': 'Actualizar Guest Additions en {vm}',
  'Stop {vm}': 'Detener {vm}',
  'Reset {vm}': 'Reiniciar {vm}',
  'Start {vm}': 'Iniciar {vm}',
  '{vm} is starting': '{vm} está iniciando',
  'Open {vm} console in a new tab': 'Abrir la consola de {vm} en una nueva pestaña',
  'Open console for {vm}': 'Abrir la consola de {vm}',
  'Live console': 'Consola en vivo',
  Machine: 'Máquina',
  'This machine is powered off.': 'Esta máquina está apagada.',
  'This machine stopped unexpectedly (aborted). Start it to boot again.':
    'Esta máquina se detuvo de forma inesperada (abortada). Iníciela para arrancar de nuevo.',
  'This machine is suspended — start it to resume exactly where it left off.':
    'Esta máquina está suspendida: iníciela para reanudar donde quedó.',
  'The guest did not respond to the shutdown signal. You can force power off.':
    'El sistema invitado no respondió a la señal de apagado. Puede forzar el apagado.',
  'Dismiss shutdown notice for {vm}': 'Descartar el aviso de apagado de {vm}',
  Configured: 'Configurado',
  Session: 'Sesión',
  Memory: 'Memoria',
  Disk: 'Disco',
  Network: 'Red',
  active: 'activo',
  'not detected': 'no detectado',
  'console attached': 'consola conectada',

  // Guest Additions update modal
  'Guest username': 'Usuario de la máquina virtual',
  'Guest password': 'Contraseña de la máquina virtual',
  'Confirm password': 'Confirmar contraseña',
  'Caps Lock is on.': 'Bloq Mayús está activado.',
  'The passwords do not match.': 'Las contraseñas no coinciden.',
  // ----- Serial-console terminal -----
  Terminal: 'Terminal',
  'serial · Linux': 'serial · Linux',
  connected: 'conectado',
  connecting: 'conectando',
  disconnected: 'desconectado',
  'A serial console gives you a shell in a tab, no GUI window.':
    'Una consola serial le da una shell en una pestaña, sin ventana gráfica.',
  'Enable serial terminal': 'Habilitar terminal serial',
  'Disable serial terminal': 'Deshabilitar terminal serial',
  'Power off the VM to enable the serial terminal.':
    'Apague la máquina para habilitar la terminal serial.',
  'Start the VM to use the terminal.': 'Encienda la máquina para usar la terminal.',
  'Open terminal': 'Abrir terminal',
  'Enable login (getty)': 'Habilitar inicio de sesión (getty)',
  'Turns on a login prompt on the serial port. Needs a root or sudo account.':
    'Activa un inicio de sesión en el puerto serial. Requiere una cuenta root o con sudo.',
  'Enable login': 'Habilitar inicio de sesión',
  terminal: 'terminal',
  'serial terminal': 'terminal serial',
  'Open {vm} terminal in a new tab': 'Abrir la terminal de {vm} en una pestaña nueva',
  'The serial terminal is only available for Linux guests.':
    'La terminal serial solo está disponible para guests Linux.',
  'The terminal is connected but the guest is not responding.':
    'La terminal está conectada pero el guest no responde.',
  'Activate it with a guest account (root or sudo). It is used once.':
    'Actívela con una cuenta de la máquina virtual (root o con sudo). Se usa una sola vez.',
  'Activate terminal': 'Activar terminal',
  'Loading…': 'Cargando…',
  cancel: 'cancelar',
  close: 'cerrar',
  // Sentence-case variants for modal action buttons (tv-btn / modal pairs);
  // the lowercase keys above stay for mono row actions and icon aria-labels.
  Cancel: 'Cancelar',
  Close: 'Cerrar',
  'updating…': 'actualizando…',
  Update: 'Actualizar',
  'Runs the installer inside the guest over VirtualBox guest control. Requires a running Linux guest with Guest Additions already active. Use root, or a user with sudo — credentials are used once and never stored.':
    'Ejecuta el instalador dentro del guest mediante guest control de VirtualBox. Requiere un guest Linux en ejecución con Guest Additions ya activo. Use root, o un usuario con sudo — las credenciales se usan una sola vez y nunca se almacenan.',

  // ----- GuestControlPanel -----
  'Guest control': 'Control del guest',
  'run commands · copy files out': 'ejecutar comandos · copiar archivos',
  'Run a command inside this VM or copy a file out of it. Needs a running Linux guest with Guest Additions active and a guest login (used once for this session, never stored).':
    'Ejecute un comando dentro de esta VM o copie un archivo desde ella. Requiere un guest Linux en marcha con Guest Additions activo y un inicio de sesión del guest (se usa una vez en esta sesión y nunca se guarda).',
  'Run in guest': 'Ejecutar en el guest',
  'Command to run': 'Comando a ejecutar',
  Run: 'Ejecutar',
  'Running…': 'Ejecutando…',
  'exit code': 'código de salida',
  'Command output': 'Salida del comando',
  '(no output)': '(sin salida)',
  '(output truncated)': '(salida truncada)',
  'Copy from guest': 'Copiar desde el guest',
  'Guest file path': 'Ruta del archivo en el guest',
  'Choose host folder': 'Elegir carpeta del anfitrión',
  'Copying…': 'Copiando…',
  'Copied to': 'Copiado en',
  'Enter the guest username and password first.':
    'Primero ingrese el usuario y la contraseña del guest.',
  'Enter a command to run (an absolute path, for example /usr/bin/uptime).':
    'Ingrese un comando a ejecutar (una ruta absoluta, por ejemplo /usr/bin/uptime).',
  'Enter the guest file path to copy out.':
    'Ingrese la ruta del archivo del guest que desea copiar.',
  'Choose a host folder to copy the file into.':
    'Elija una carpeta del anfitrión donde copiar el archivo.',
  // ----- FilesPanel -----
  Files: 'Archivos',
  'shared folders': 'carpetas compartidas',
  'No folders shared yet. Pick a host folder to make it appear inside this VM.':
    'Aún no hay carpetas compartidas. Elija una carpeta del anfitrión para que aparezca dentro de esta VM.',
  session: 'sesión',
  persistent: 'persistente',
  'Add folder': 'Agregar carpeta',
  'Choose a folder…': 'Elija una carpeta…',
  'Working…': 'Trabajando…',
  Remove: 'Quitar',
  'Shared only until this VM reboots (the VM is running)':
    'Compartida solo hasta que la VM se reinicie (la VM está en ejecución)',
  'Persistent share; survives reboots': 'Compartición persistente; sobrevive a los reinicios',
  'Drop files to send to the VM': 'Suelte archivos para enviarlos a la VM',

  // ----- NetworkPanel -----
  'adapter mode': 'modo de adaptador',
  Bridged: 'Puente',
  'Host-only': 'Solo anfitrión',
  'No enabled network adapters on this VM.': 'No hay adaptadores de red habilitados en esta VM.',
  'No bridge-able host interface found.': 'No se encontró una interfaz del anfitrión para modo puente.',
  'No host-only adapter — create one in VirtualBox first.':
    'No hay adaptador solo-anfitrión — cree uno en VirtualBox primero.',
  Apply: 'Aplicar',
  'Applying…': 'Aplicando…',
  'Port forwarding': 'Redirección de puertos',
  'Rule name': 'Nombre de la regla',
  Protocol: 'Protocolo',
  'Host port': 'Puerto del anfitrión',
  'Guest port': 'Puerto del guest',
  'Host IP (optional)': 'IP del anfitrión (opcional)',
  'Add rule': 'Agregar regla',
  'Remove rule': 'Quitar regla',
  '* = bound to all host interfaces': '* = enlazado a todas las interfaces del anfitrión',
  name: 'nombre',
  'host port': 'puerto del anfitrión',
  'guest port': 'puerto del guest',
  Cable: 'Cable',
  Connect: 'Conectar',
  Disconnect: 'Desconectar',
  'Disconnecting simulates unplugging the cable; works on running and stopped VMs.':
    'Desconectar simula desenchufar el cable; funciona en máquinas en ejecución y detenidas.',

  // ----- USB panel -----
  USB: 'USB',
  'device passthrough': 'paso de dispositivos',
  'USB device': 'dispositivo USB',
  Attach: 'Conectar',
  Available: 'disponible',
  Busy: 'ocupado',
  Captured: 'capturado',
  Unavailable: 'no disponible',
  // Tooltips explaining each USB capture state.
  'Free to attach': 'Libre para conectar',
  'In use by the host or another program': 'En uso por el anfitrión u otro programa',
  'Attached to a virtual machine': 'Conectado a una máquina virtual',
  'Cannot be captured': 'No se puede capturar',
  'No USB devices detected on the host.': 'No se detectaron dispositivos USB en el anfitrión.',
  'This VM has no USB controller enabled. Power the VM off and enable USB in its settings — the controller cannot be turned on while the VM is running.':
    'Esta VM no tiene un controlador USB habilitado. Apague la VM y habilite USB en su configuración: el controlador no se puede activar mientras la VM está en marcha.',
  'USB 2.0 and 3.0 passthrough needs the Oracle VirtualBox Extension Pack. Install it to attach most devices.':
    'El paso de USB 2.0 y 3.0 requiere el Oracle VirtualBox Extension Pack. Instálelo para conectar la mayoría de los dispositivos.',
  // Server-side USB messages (localized via server-message lookup).
  'USB device attached to the VM.': 'Dispositivo USB conectado a la VM.',
  'USB device detached from the VM.': 'Dispositivo USB desconectado de la VM.',
  'The VM must be running to attach or detach USB devices.':
    'La VM debe estar en marcha para conectar o desconectar dispositivos USB.',
  'Invalid USB device identifier.': 'Identificador de dispositivo USB no válido.',
  'This VM has no USB controller enabled. Power the VM off and enable USB in its settings first.':
    'Esta VM no tiene un controlador USB habilitado. Apague la VM y habilite USB en su configuración primero.',
  'Attaching this device needs the Oracle VirtualBox Extension Pack. Install it, then try again.':
    'Conectar este dispositivo requiere el Oracle VirtualBox Extension Pack. Instálelo y vuelva a intentarlo.',
  'The host USB service is unavailable. Reinstall VirtualBox so its USB driver is registered, then reconnect the device.':
    'El servicio USB del anfitrión no está disponible. Reinstale VirtualBox para registrar su controlador USB y vuelva a conectar el dispositivo.',

  // Hardware panel
  Hardware: 'Hardware',
  'vCPU · memory': 'vCPU · memoria',
  'Power off the VM to change hardware.': 'Apague la VM para cambiar el hardware.',
  'Hardware information unavailable.': 'Información de hardware no disponible.',

  // Storage panel
  Storage: 'Almacenamiento',
  'disk size': 'tamaño de disco',
  'Storage information unavailable.': 'Información de almacenamiento no disponible.',
  'No hard disks attached to this VM.': 'No hay discos duros conectados a esta VM.',
  'New size (GB)': 'Nuevo tamaño (GB)',
  Resize: 'Redimensionar',
  'Resizing…': 'Redimensionando…',
  'Add a disk': 'Agregar un disco',
  'a new VDI attached to a free SATA port': 'un nuevo VDI conectado a un puerto SATA libre',
  'New disk size (GB)': 'Tamaño del nuevo disco (GB)',
  'Add disk': 'Agregar disco',
  'Adding…': 'Agregando…',
  'DVD drive': 'Unidad de DVD',
  empty: 'vacía',
  'no optical drive': 'sin unidad óptica',
  'Mount ISO': 'Montar ISO',
  'Change ISO': 'Cambiar ISO',
  Eject: 'Expulsar',
  Detach: 'Desconectar',
  Delete: 'Eliminar',
  'free port': 'puerto libre',
  'new disk attached': 'nuevo disco conectado',
  'file kept': 'archivo conservado',
  'file erased': 'archivo borrado',
  'Detach {name}': 'Desconectar {name}',
  'Delete {name}': 'Eliminar {name}',
  'Detach "{name}" from this VM? The disk file is kept and can be re-attached later.':
    '¿Desconectar "{name}" de esta VM? El archivo del disco se conserva y se puede volver a conectar más tarde.',
  'Permanently delete "{name}"? Its disk image file will be removed from this computer. This cannot be undone.':
    '¿Eliminar "{name}" de forma permanente? El archivo de imagen del disco se eliminará de este equipo. Esta acción no se puede deshacer.',
  'This disk cannot be resized.': 'Este disco no se puede redimensionar.',
  'Resizing only enlarges the virtual disk. Expand the partition inside the guest OS to use the new space.':
    'Redimensionar solo agranda el disco virtual. Expanda la partición dentro del sistema guest para usar el espacio nuevo.',
  'Fixed-size disks cannot be resized.': 'Los discos de tamaño fijo no se pueden redimensionar.',
  'This disk has snapshots. Delete them before resizing.':
    'Este disco tiene instantáneas. Elimínelas antes de redimensionar.',
  'Power off the VM to resize its disks.': 'Apague la VM para redimensionar sus discos.',
  now: 'ahora',

  // ----- SnapshotsPanel -----
  Snapshots: 'Instantáneas',
  'restore points': 'puntos de restauración',
  'No snapshots yet. Take one before you experiment, then roll back anytime.':
    'Aún no hay instantáneas. Cree una antes de experimentar y vuelva atrás cuando quiera.',
  'Snapshot name (optional)': 'Nombre de la instantánea (opcional)',
  'Take snapshot': 'Crear instantánea',
  restore: 'restaurar',
  current: 'actual',
  'Power off and roll the VM back to this snapshot':
    'Apagar y revertir la VM a esta instantánea',
  'Delete this snapshot': 'Eliminar esta instantánea',
  'the VM': 'la VM',
  'Restore "{name}"? This powers off {vm} and rolls it back — anything not captured in a snapshot is lost.':
    '¿Restaurar "{name}"? Esto apaga {vm} y la revierte — se pierde todo lo que no esté capturado en una instantánea.',
  'Delete snapshot "{name}"? Its changes merge into the parent and it cannot be recovered.':
    '¿Eliminar la instantánea "{name}"? Sus cambios se fusionan con el padre y no se puede recuperar.',
  'Reset will forcibly restart "{name}" and may cause data loss. This is not a graceful shutdown. Are you sure you want to continue?':
    'Reiniciar forzará el arranque de "{name}" y puede causar pérdida de datos. No es un apagado ordenado. ¿Seguro que desea continuar?',

  // ----- GuestDropZone -----
  'Guest credentials': 'Credenciales del guest',
  Send: 'Enviar',
  'Guest credentials required.': 'Se requieren credenciales del guest.',
  Dismiss: 'Descartar',
  'Drop to send to {vm}': 'Suelte para enviar a {vm}',
  'This VM': 'Esta VM',
  'the file': 'el archivo',
  '{n} files': '{n} archivos',
  '{vm} has no shared folder, so {files} will be copied in over VirtualBox guest control. Enter a guest username and password — used once and reused only for this session, never stored. Tip: add a shared folder to skip this next time.':
    '{vm} no tiene carpeta compartida, así que {files} se copiará mediante guest control de VirtualBox. Ingrese un usuario y una contraseña del guest — se usan una sola vez y se reutilizan solo en esta sesión, nunca se almacenan. Sugerencia: agregue una carpeta compartida para omitir esto la próxima vez.',

  // ----- ScreenConsole -----
  Telemetry: 'Telemetría',
  '● Guest Additions active': '● Guest Additions activo',
  'Guest Additions not detected': 'Guest Additions no detectado',
  'connecting…': 'conectando…',
  'Send Ctrl+Alt+Del': 'Enviar Ctrl+Alt+Del',
  'Close console': 'Cerrar consola',
  'Console for {vm}': 'Consola de {vm}',
  Clipboard: 'Portapapeles',
  'Shared clipboard mode': 'Modo de portapapeles compartido',
  off: 'desactivado',
  disabled: 'desactivado',
  'host → guest': 'anfitrión → guest',
  'guest → host': 'guest → anfitrión',
  bidirectional: 'bidireccional',
  'Collapse panel': 'Contraer panel',
  'Expand panel': 'Expandir panel',
  'Toggle telemetry panel': 'Alternar panel de telemetría',
  'Connection failed. Is the VM running?':
    'La conexión falló. ¿La VM está en ejecución?',

  // ----- AgentView (System) -----
  'The local TabVM agent bound to 127.0.0.1, and its VirtualBox link.':
    'El agente local de TabVM enlazado a 127.0.0.1, y su vínculo con VirtualBox.',
  Runtime: 'Entorno de ejecución',
  Status: 'Estado',
  healthy: 'saludable',
  'checking…': 'comprobando…',
  unreachable: 'inaccesible',
  Uptime: 'Tiempo activo',
  Bound: 'Enlazado',
  found: 'encontrado',
  'not found': 'no encontrado',
  Version: 'Versión',
  'Latest available release': 'Última versión disponible',
  'update available': 'actualización disponible',
  'Local state': 'Estado local',
  Store: 'Almacén',
  ready: 'listo',
  unavailable: 'no disponible',
  Schema: 'Esquema',

  // ----- SharedFolders -----
  'Shared folders': 'Carpetas compartidas',
  'No shared folders yet.': 'Aún no hay carpetas compartidas.',
  'Only for the current session': 'Solo para la sesión actual',
  'Remove shared folder {name}': 'Quitar la carpeta compartida {name}',
  'Share name (e.g. labshare)': 'Nombre del recurso (p. ej. labshare)',
  'Share name': 'Nombre del recurso',
  'Host path (e.g. C:\\labs\\share)': 'Ruta del anfitrión (p. ej. C:\\labs\\share)',
  'Host path': 'Ruta del anfitrión',
  'Share folder': 'Compartir carpeta',

  // ----- ConsolePreview -----
  'Open live console': 'Abrir consola en vivo',
  'console unavailable': 'consola no disponible',
  'click to attach keyboard + mouse': 'clic para conectar teclado + ratón',

  // ----- CreateVmWizard -----
  'New VM': 'Nueva VM',
  'Create a virtual machine': 'Crear una máquina virtual',
  'New virtual machine': 'Nueva máquina virtual',
  'Import image (.ova)': 'Importar imagen (.ova)',
  'Install from ISO': 'Instalar desde ISO',
  'Other OS (manual install)': 'Otro SO (instalación manual)',
  'Import a prebuilt appliance that already has Guest Additions. Best for Kali. One click, no install.':
    'Importe un appliance prehorneado que ya tiene Guest Additions. Ideal para Kali. Un clic, sin instalación.',
  'Create a VM and run an automated Ubuntu, Debian or Windows install with Guest Additions included. Kali is not supported here.':
    'Cree una VM y ejecute una instalación automatizada de Ubuntu, Debian o Windows con Guest Additions incluido. Kali no está soportado aquí.',
  'Create a VM with any bootable ISO attached. You install the OS yourself in the console; Guest Additions are not installed automatically.':
    'Cree una VM con cualquier ISO arrancable adjunta. Usted instala el sistema operativo en la consola; Guest Additions no se instala automáticamente.',
  'VM name': 'Nombre de la VM',
  'Choose .ova/.ovf file': 'Elegir archivo .ova/.ovf',
  'No file selected': 'Ningún archivo seleccionado',
  'Choose .iso file': 'Elegir archivo .iso',
  'Operating system': 'Sistema operativo',
  'Memory (MB)': 'Memoria (MB)',
  CPUs: 'CPUs',
  'Disk (GB)': 'Disco (GB)',
  Import: 'Importar',
  Create: 'Crear',
  Done: 'Listo',
  Back: 'Volver',
  'Importing the appliance… this can take several minutes.':
    'Importando el appliance… esto puede tardar varios minutos.',
  'Creating the VM and preparing the automated install…':
    'Creando la VM y preparando la instalación automatizada…',
  'Creating the VM and attaching the installer ISO…':
    'Creando la VM y adjuntando la ISO de instalación…',
  'The VM is ready. Start it from the list.':
    'La VM está lista. Iníciela desde la lista.',
  'Start the VM to run the install; watch it in the console.':
    'Inicie la VM para ejecutar la instalación; obsérvela en la consola.',
  'Start the VM and install the OS yourself in the console.':
    'Inicie la VM e instale el sistema operativo usted mismo en la consola.',
  'The creation job is no longer available. The agent may have restarted; check the machine list before retrying.':
    'El trabajo de creación ya no está disponible. Es posible que el agente se haya reiniciado; revise la lista de máquinas antes de reintentar.',
  'Lost contact with the agent while creating the VM. Check the machine list before retrying.':
    'Se perdió el contacto con el agente durante la creación de la VM. Revise la lista de máquinas antes de reintentar.',

  // ----- Clone VM (MachinesView) -----
  clone: 'clonar',
  'Clone VM': 'Clonar VM',
  Clone: 'Clonar',
  'New VM name': 'Nombre de la nueva VM',
  'Clone type': 'Tipo de clon',
  'Full clone (independent copy)': 'Clon completo (copia independiente)',
  'Linked clone (faster, shares the source disk)':
    'Clon enlazado (más rápido, comparte el disco de origen)',
  'A linked clone requires the source VM to have at least one snapshot. Take a snapshot first if it has none.':
    'Un clon enlazado requiere que la VM de origen tenga al menos una instantánea. Cree una instantánea primero si no tiene ninguna.',
  'Cloning the VM… a full clone copies the disks and can take several minutes.':
    'Clonando la VM… un clon completo copia los discos y puede tardar varios minutos.',
  'Create a copy of "{name}". The source machine must be powered off.':
    'Cree una copia de "{name}". La máquina de origen debe estar apagada.',
  'The clone job is no longer available. The agent may have restarted; check the machine list before retrying.':
    'El trabajo de clonación ya no está disponible. Es posible que el agente se haya reiniciado; revise la lista de máquinas antes de reintentar.',
  'Lost contact with the agent while cloning the VM. Check the machine list before retrying.':
    'Se perdió el contacto con el agente durante la clonación de la VM. Revise la lista de máquinas antes de reintentar.',

  // ----- Export VM (MachinesView) -----
  export: 'exportar',
  'Export VM': 'Exportar VM',
  'Choose folder & export': 'Elegir carpeta y exportar',
  done: 'listo',
  'The appliance was exported.': 'El dispositivo se exportó.',
  'Exporting the VM… copying the disks into the appliance can take several minutes.':
    'Exportando la VM… copiar los discos al dispositivo puede tardar varios minutos.',
  'Export "{name}" to an .ova appliance. The machine must be powered off; you choose a destination folder next.':
    'Exporte "{name}" a un dispositivo .ova. La máquina debe estar apagada; a continuación elegirá una carpeta de destino.',
  'The export job is no longer available. The agent may have restarted; check the machine list before retrying.':
    'El trabajo de exportación ya no está disponible. Es posible que el agente se haya reiniciado; revise la lista de máquinas antes de reintentar.',
  'Lost contact with the agent while exporting the VM. Check the machine list before retrying.':
    'Se perdió el contacto con el agente durante la exportación de la VM. Revise la lista de máquinas antes de reintentar.',

  // ----- ActivityView -----
  'Recorded machine operations, newest first.':
    'Operaciones registradas de las máquinas, las más recientes primero.',
  'Operation log': 'Registro de operaciones',
  'Loading activity…': 'Cargando actividad…',
  'Activity is unavailable. The agent may not expose the log yet.':
    'La actividad no está disponible. Puede que el agente aún no exponga el registro.',
  'No recorded operations yet.': 'Aún no hay operaciones registradas.',
  'No additional detail was recorded.': 'No se registró ningún detalle adicional.',
  Succeeded: 'Correcto',
  Failed: 'Falló',
  'filter…': 'filtrar…',
  'Filter activity': 'Filtrar actividad',
  'Clear filter': 'Limpiar filtro',
  'No matches.': 'Sin resultados.',
  // Activity action labels (English label = key, produced by actionLabel()).
  // Labels already defined above (Install/Update Guest Additions, Delete VM,
  // Enable/Disable serial terminal, Add disk, Mount ISO, Take snapshot,
  // Clone VM, Export VM) are reused, not repeated here.
  'Start VM': 'Iniciar VM',
  'Stop VM (ACPI)': 'Detener VM (ACPI)',
  'Force power off': 'Forzar apagado',
  'Reset VM': 'Reiniciar VM',
  'Suspend VM': 'Suspender VM',
  'Create VM': 'Crear VM',
  'Clean up failed VM create': 'Limpiar creación de VM fallida',
  'Import VM': 'Importar VM',
  'Change hardware': 'Cambiar hardware',
  'Resize disk': 'Redimensionar disco',
  'Detach disk': 'Desconectar disco',
  'Eject ISO': 'Expulsar ISO',
  'Add shared folder': 'Agregar carpeta compartida',
  'Remove shared folder': 'Quitar carpeta compartida',
  'Restore snapshot': 'Restaurar instantánea',
  'Delete snapshot': 'Eliminar instantánea',
  'Change network mode': 'Cambiar modo de red',
  'Set network cable': 'Configurar cable de red',
  'Add port forwarding rule': 'Agregar regla de redirección de puertos',
  'Delete port forwarding rule': 'Eliminar regla de redirección de puertos',
  'Run command in guest': 'Ejecutar comando en el invitado',
  'Copy file from guest': 'Copiar archivo desde el invitado',
  'Transfer file to guest': 'Transferir archivo al invitado',
  'Set clipboard mode': 'Configurar modo de portapapeles',
  'Configure serial getty': 'Configurar getty serial',
  'Disable console': 'Deshabilitar consola',
  'Attach USB device': 'Conectar dispositivo USB',
  'Detach USB device': 'Desconectar dispositivo USB',

  // ----- UpdateBanner -----
  'TabVM v{version} is available': 'TabVM v{version} ya está disponible',
  Download: 'Descargar',
  'Installed with Scoop? Run `scoop update tabvm`':
    '¿Instalado con Scoop? Ejecute `scoop update tabvm`',
};

// Exact-match localization for backend-produced errors and notifications.
export const esServerExact: Record<string, string> = {
  'Invalid VM identifier.': 'Identificador de VM inválido.',
  'Invalid snapshot identifier.': 'Identificador de instantánea inválido.',
  'Invalid file name.': 'Nombre de archivo inválido.',
  'Invalid request body.': 'Cuerpo de la solicitud inválido.',
  'VirtualBox operation failed.': 'La operación de VirtualBox falló.',
  'The VM is busy or locked by another session. Wait a moment and try again.':
    'La VM está ocupada o bloqueada por otra sesión. Espere un momento e intente de nuevo.',
  'Hardware virtualization is unavailable. Check that Hyper-V or Windows memory integrity is not blocking VirtualBox.':
    'La virtualización por hardware no está disponible. Verifique que Hyper-V o la integridad de memoria de Windows no esté bloqueando VirtualBox.',
  'Not enough host memory to start the VM.':
    'No hay suficiente memoria en el anfitrión para iniciar la VM.',
  'Internal server error.': 'Error interno del servidor.',
  'Snapshot deleted.': 'Instantánea eliminada.',
  'Snapshot restored. The VM was rolled back and is powered off — start it to boot the restored state.':
    'Instantánea restaurada. La VM se revirtió y está apagada — iníciela para arrancar el estado restaurado.',
  'Snapshot name must be 1-100 characters and cannot start with a dash.':
    'El nombre de la instantánea debe tener entre 1 y 100 caracteres y no puede comenzar con un guion.',
  'Snapshot name contains unsupported characters.':
    'El nombre de la instantánea contiene caracteres no admitidos.',
  'Snapshot description must be 512 characters or fewer.':
    'La descripción de la instantánea debe tener 512 caracteres o menos.',
  'The VM did not power off in time to restore the snapshot.':
    'La VM no se apagó a tiempo para restaurar la instantánea.',
  'Network mode must be one of: nat, bridged, hostonly.':
    'El modo de red debe ser uno de: nat, bridged, hostonly.',
  'Network adapter slot must be between 1 and 8.':
    'La ranura del adaptador de red debe estar entre 1 y 8.',
  'A host interface is required for bridged and host-only modes.':
    'Se requiere una interfaz del anfitrión para los modos puente y solo-anfitrión.',
  'Host interface name contains unsupported characters.':
    'El nombre de la interfaz del anfitrión contiene caracteres no admitidos.',
  'Protocol must be tcp or udp.': 'El protocolo debe ser tcp o udp.',
  'Rule name must be 1-64 characters, cannot contain a comma, and cannot start with a dash.':
    'El nombre de la regla debe tener entre 1 y 64 caracteres, no puede contener una coma ni comenzar con un guion.',
  'Host port must be between 1 and 65535.':
    'El puerto del anfitrión debe estar entre 1 y 65535.',
  'Guest port must be between 1 and 65535.':
    'El puerto del guest debe estar entre 1 y 65535.',
  'Host IP is not a valid IP address.':
    'La IP del anfitrión no es una dirección IP válida.',
  'Guest IP is not a valid IP address.':
    'La IP del guest no es una dirección IP válida.',
  'This VM has no shared folder, so copying a file in needs the guest username and password.':
    'Esta VM no tiene carpeta compartida, así que copiar un archivo requiere el usuario y la contraseña del guest.',
  'The VM must be running to copy files into it.':
    'La VM debe estar en ejecución para copiar archivos en ella.',
  'No file content was uploaded.': 'No se cargó ningún contenido de archivo.',
  'No file was uploaded.': 'No se cargó ningún archivo.',
  'Could not read the uploaded file.': 'No se pudo leer el archivo cargado.',
  'Could not open the folder picker.': 'No se pudo abrir el selector de carpetas.',
  'Invalid or oversized upload (max 256 MB).':
    'Carga inválida o demasiado grande (máx. 256 MB).',
  'Could not copy the file into the guest. Check the username/password and that the guest is a running Linux VM with Guest Additions active.':
    'No se pudo copiar el archivo al guest. Verifique el usuario/contraseña y que el guest sea una VM Linux en ejecución con Guest Additions activo.',
  'Another operation is already in progress for this VM.':
    'Ya hay otra operación en curso para esta VM.',
  'Another lifecycle operation is already in progress for this VM.':
    'Ya hay otra operación de ciclo de vida en curso para esta VM.',
  'Shared folder name must be 1-64 characters using letters, digits, dot, dash or underscore.':
    'El nombre de la carpeta compartida debe tener entre 1 y 64 caracteres con letras, dígitos, punto, guion o guion bajo.',
  'Host path is required.': 'La ruta del anfitrión es obligatoria.',
  'Host path must be an absolute path.': 'La ruta del anfitrión debe ser una ruta absoluta.',
  'Host path does not exist or is not accessible.':
    'La ruta del anfitrión no existe o no es accesible.',
  'Host path must be a directory.': 'La ruta del anfitrión debe ser un directorio.',
  'Shared folder not found.': 'Carpeta compartida no encontrada.',
  'Guest username and password are required.':
    'El usuario y la contraseña del guest son obligatorios.',
  'Guest username contains unsupported characters.':
    'El usuario del guest contiene caracteres no admitidos.',
  'Could not update Guest Additions inside the guest. Check the username/password, that the account is root or has sudo, and that the guest is a running Linux VM with Guest Additions already active.':
    'No se pudo actualizar Guest Additions dentro del guest. Verifique el usuario/contraseña, que la cuenta sea root o tenga sudo, y que el guest sea una VM Linux en ejecución con Guest Additions ya activo.',
  'Guest Additions disc inserted. Run the installer inside the VM to finish setup.':
    'Disco de Guest Additions insertado. Ejecute el instalador dentro de la VM para finalizar la instalación.',
  'This VM has no optical (DVD) drive to insert the Guest Additions disc into. Add a DVD drive in VirtualBox, then try again.':
    'Esta VM no tiene una unidad óptica (DVD) donde insertar el disco de Guest Additions. Agregue una unidad de DVD en VirtualBox e intente de nuevo.',
  'Guest Additions update started. The VM installs it in the background and reboots automatically in 1–3 minutes — reopen the console once it is back. (Guest log: /var/log/tabvm-ga.log)':
    'Actualización de Guest Additions iniciada. La VM la instala en segundo plano y se reinicia automáticamente en 1–3 minutos — reabra la consola cuando vuelva. (Registro del guest: /var/log/tabvm-ga.log)',
  'VM name must be 1-64 characters using letters, digits, space, dot, dash or underscore.':
    'El nombre de la VM debe tener entre 1 y 64 caracteres con letras, dígitos, espacio, punto, guion o guion bajo.',
  'Unsupported OS type for unattended install.':
    'Tipo de SO no soportado para instalación automatizada.',
  'Unsupported OS type for manual install.':
    'Tipo de SO no soportado para instalación manual.',
  'The installer must be a .iso file.': 'El instalador debe ser un archivo .iso.',
  'The appliance must be a .ova or .ovf file.': 'El appliance debe ser un archivo .ova o .ovf.',
  'A host file path is required.': 'Se requiere la ruta de un archivo del anfitrión.',
  'The path must be absolute.': 'La ruta debe ser absoluta.',
  "The path must not contain '..' segments.": "La ruta no debe contener segmentos '..'.",
  'The file does not exist or is not accessible.': 'El archivo no existe o no es accesible.',
  'The path must be a file, not a directory.': 'La ruta debe ser un archivo, no un directorio.',
  'A guest password is required.': 'Se requiere una contraseña del guest.',
  'Memory must be between 512 MB and 65536 MB.':
    'La memoria debe estar entre 512 MB y 65536 MB.',
  'CPU count must be between 1 and 16.': 'La cantidad de CPU debe estar entre 1 y 16.',
  'Disk size must be between 8 GB and 512 GB.':
    'El tamaño del disco debe estar entre 8 GB y 512 GB.',
  'Could not determine the new VM identifier.':
    'No se pudo determinar el identificador de la nueva VM.',
  'Could not open the file picker.': 'No se pudo abrir el selector de archivos.',
  'The VM is running. Power it off before cloning it.':
    'La VM está en marcha. Apáguela antes de clonarla.',
  'A linked clone requires a snapshot. Take a snapshot of the source VM first, then clone it.':
    'Un clon enlazado requiere una instantánea. Cree una instantánea de la VM de origen primero y luego clónela.',
};

// Pattern localization for backend messages carrying names or paths. $1, $2… map
// to capture groups. More specific patterns must come first.
export const esServerPatterns: Array<[RegExp, string]> = [
  [
    /^VirtualBox operation failed \(exit code (\d+)\)\.$/,
    'La operación de VirtualBox falló (código de salida $1).',
  ],
  [/^Snapshot "(.+)" created\.$/, 'Instantánea "$1" creada.'],
  [/^Adapter (\d+) switched to (.+?) \((.+)\)\.$/, 'Adaptador $1 cambiado a $2 ($3).'],
  [/^Adapter (\d+) switched to (.+)\.$/, 'Adaptador $1 cambiado a $2.'],
  [
    /^Forwarding (.+):(\d+) -> guest:(\d+) added on adapter (\d+)\.$/,
    'Redirección $1:$2 -> guest:$3 agregada en el adaptador $4.',
  ],
  [
    /^Forwarding rule "(.+)" removed from adapter (\d+)\.$/,
    'Regla de redirección "$1" quitada del adaptador $2.',
  ],
  [/^Adapter (\d+) cable connected\.$/, 'Adaptador $1: cable conectado.'],
  [/^Adapter (\d+) cable disconnected\.$/, 'Adaptador $1: cable desconectado.'],
  [/^Adapter (\d+) is not enabled on this VM\.$/, 'El adaptador $1 no está habilitado en esta VM.'],
  [
    /^Adapter (\d+) must be in NAT mode to add a port-forwarding rule\.$/,
    'El adaptador $1 debe estar en modo NAT para agregar una regla de redirección de puertos.',
  ],
  [
    /^Adapter (\d+) already has a rule named "(.+)"\.$/,
    'El adaptador $1 ya tiene una regla llamada "$2".',
  ],
  [
    /^Host port (\d+)\/(tcp|udp) is already forwarded on this VM\.$/,
    'El puerto del anfitrión $1/$2 ya está redirigido en esta VM.',
  ],
  [
    /^"(.+)" is in shared folder "(.+)" \(guest: (.+)\)\.$/,
    '"$1" está en la carpeta compartida "$2" (guest: $3).',
  ],
  [/^"(.+)" copied into the guest at (.+)\.$/, '"$1" copiado al guest en $2.'],
  [
    /^Shared folder "(.+)" added for the current session \(VM is running\)\.$/,
    'Carpeta compartida "$1" agregada para la sesión actual (la VM está en ejecución).',
  ],
  [/^Shared folder "(.+)" added\.$/, 'Carpeta compartida "$1" agregada.'],
  [/^Shared folder "(.+)" removed\.$/, 'Carpeta compartida "$1" quitada.'],
  [/^"(.+)" imported and ready to start\.$/, '"$1" importada y lista para iniciar.'],
  [/^Exported to (.+)$/, 'Exportado a $1'],
  [
    /^"(.+)" created\. Start it to run the automated install with Guest Additions\.$/,
    '"$1" creada. Iníciela para ejecutar la instalación automatizada con Guest Additions.',
  ],
  [
    /^"(.+)" created\. Start it and install the OS from the attached ISO\.$/,
    '"$1" creada. Iníciela e instale el sistema operativo desde la ISO adjunta.',
  ],
  [
    /^Disk resized to (\d+) MB\. Expand the partition inside the guest to use the new space\.$/,
    'Disco redimensionado a $1 MB. Expanda la partición dentro del guest para usar el espacio nuevo.',
  ],
  [
    /^Disks can only grow\. Enter a size larger than the current (\d+) MB\.$/,
    'Los discos solo pueden crecer. Ingrese un tamaño mayor que los $1 MB actuales.',
  ],
  [
    /^Added a (\d+) MB disk on (.+) port (\d+)\.$/,
    'Se agregó un disco de $1 MB en $2 puerto $3.',
  ],
  [
    /^Only VDI and VHD disks can be resized \(this one is (.+)\)\.$/,
    'Solo los discos VDI y VHD se pueden redimensionar (este es $1).',
  ],
  [
    /^Hardware updated: (\d+) vCPU, (\d+) MB memory\.$/,
    'Hardware actualizado: $1 vCPU, $2 MB de memoria.',
  ],
  [
    /^Disk detached from the VM\. Its file was kept and can be re-attached later\.$/,
    'Disco desconectado de la VM. Su archivo se conservó y se puede volver a conectar más tarde.',
  ],
  [
    /^Disk detached and its file permanently deleted\.$/,
    'Disco desconectado y su archivo eliminado de forma permanente.',
  ],
  [
    /^This disk has snapshots\. Delete them before deleting the disk\.$/,
    'Este disco tiene instantáneas. Elimínelas antes de eliminar el disco.',
  ],
  [/^That disk is not attached to this VM\.$/, 'Ese disco no está conectado a esta VM.'],
];
