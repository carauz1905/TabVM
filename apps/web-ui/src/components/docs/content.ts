import { useLang, type Lang } from '../../i18n/i18n';

// Docs prose lives here, separate from the app's UI-string dictionary, because
// it is long-form content. Each section component pulls its text from the
// current-language object and embeds the interactive demos itself.

export interface Step {
  title: string;
  body: string;
}

export interface Faq {
  q: string;
  a: string;
}

export interface DocsStrings {
  tagline: string;
  sectionNav: string;
  tipLabel: string;
  sections: {
    welcome: string;
    quickStart: string;
    operate: string;
    terminal: string;
    hardware: string;
    storage: string;
    files: string;
    snapshots: string;
    network: string;
    create: string;
    personalize: string;
    troubleshoot: string;
  };
  welcome: {
    lead: string;
    cards: Step[];
    tryHint: string;
  };
  quickStart: {
    lead: string;
    steps: Step[];
  };
  operate: {
    lead: string;
    startStop: Step;
    console: Step;
    tip: string;
  };
  terminal: {
    lead: string;
    enable: Step;
    open: Step;
    activate: Step;
    tip: string;
  };
  hardware: {
    lead: string;
    edit: Step;
    limits: Step;
    tip: string;
  };
  storage: {
    lead: string;
    resize: Step;
    add: Step;
    remove: Step;
    limits: Step;
    tip: string;
  };
  files: {
    lead: string;
    share: Step;
    drop: Step;
    tip: string;
  };
  snapshots: {
    lead: string;
    take: Step;
    restore: Step;
    tip: string;
  };
  network: {
    lead: string;
    modes: Step[];
    forwarding: Step;
    tip: string;
  };
  create: {
    lead: string;
    importCard: Step;
    installCard: Step;
    manualCard: Step;
    cloneCard: Step;
    exportCard: Step;
    note: string;
  };
  personalize: {
    lead: string;
    theme: Step;
    lang: Step;
    accent: Step;
    tryHint: string;
  };
  troubleshoot: {
    lead: string;
    gaManual: {
      title: string;
      intro: string;
      commands: string[];
      after: string;
    };
    faqs: Faq[];
  };
  credits: {
    by: string;
    name: string;
  };
}

const en: DocsStrings = {
  tagline: 'every VM. one tab.',
  sectionNav: 'On this page',
  tipLabel: 'Tip',
  sections: {
    welcome: 'Welcome',
    quickStart: 'Quick start',
    operate: 'Operate a VM',
    terminal: 'Serial terminal',
    hardware: 'CPU & memory',
    storage: 'Disk storage',
    files: 'Files & sharing',
    snapshots: 'Snapshots',
    network: 'Networking',
    create: 'Create a VM',
    personalize: 'Personalize',
    troubleshoot: 'Troubleshooting',
  },
  welcome: {
    lead: 'TabVM turns your local VirtualBox machines into browser tabs. Install nothing but VirtualBox — everything else runs from this window, on your own computer, at 127.0.0.1.',
    cards: [
      { title: 'Local and private', body: 'The agent binds only to 127.0.0.1. Your VMs never leave your machine, and nothing is sent to the cloud.' },
      { title: 'Everything in one view', body: 'Start, stop, watch the screen, share files, take snapshots and switch networks — without ever opening the VirtualBox window.' },
      { title: 'Built for learning', body: 'Import an appliance in one click, drive it from a guided console, and follow a manual that shows you exactly what each control does.' },
    ],
    tryHint: 'This manual is interactive. Buttons you see in demos are the real TabVM controls — click them.',
  },
  quickStart: {
    lead: 'Three steps from a fresh install to a running machine.',
    steps: [
      { title: 'Install & open', body: 'Run the TabVM installer, then launch it. Your browser opens the dashboard at 127.0.0.1 — no login, no setup.' },
      { title: 'Add a machine', body: 'Click New VM. Import a ready appliance (fastest) or install a fresh OS from an ISO with Guest Additions baked in.' },
      { title: 'Start & connect', body: 'Press start on the machine, then Open console to see and control its screen right inside the tab.' },
    ],
  },
  operate: {
    lead: 'Each machine in the list has a compact set of controls. They mirror exactly what you will use every day.',
    startStop: {
      title: 'Start, stop, reset',
      body: 'A stopped machine shows start. A running one shows stop (a clean shutdown, like pressing the power button) and reset (a hard restart — use it only when the guest is stuck).',
    },
    console: {
      title: 'The live console',
      body: 'Open console attaches to the machine screen over VirtualBox and streams it here. Click into it to send keyboard and mouse; open it full-screen in a new tab when you need room.',
    },
    tip: 'Selecting a machine — running or powered off — opens its focus panel below, where Files, Network and Snapshots live.',
  },
  terminal: {
    lead: 'Open a text shell to a Linux machine right in a tab — no graphical window. The button appears only on Linux machines, and it runs over the machine’s serial port, so no network or firewall is involved.',
    enable: {
      title: 'Turn it on',
      body: 'The first time, power the machine off and click Enable serial terminal — that wires up the serial port. Then start the machine.',
    },
    open: {
      title: 'Open it',
      body: 'Click the terminal button on the machine’s row. It opens full-screen in a new tab, just like the console.',
    },
    activate: {
      title: 'If the screen stays blank',
      body: 'A blank terminal means no one is answering the port yet. A small card asks once for a machine account (root, or a user with sudo) and turns on the login — then the prompt appears.',
    },
    tip: 'The serial terminal never uses the network, so firewall rules and policies can’t block it — handy on locked-down machines.',
  },
  hardware: {
    lead: 'Give a machine more (or less) power. The Hardware panel in a machine’s focus view edits its virtual CPUs and memory — no need to open VirtualBox.',
    edit: {
      title: 'Change vCPU and memory',
      body: 'Set the number of virtual CPUs and the memory in megabytes, then press Apply. The change is written straight into the machine’s configuration.',
    },
    limits: {
      title: 'Bounded by your host',
      body: 'The inputs never let you assign more processors or memory than your computer actually has, so you cannot leave a machine unbootable.',
    },
    tip: 'Hardware can only change while the machine is powered OFF — VirtualBox will not resize a running VM. If the fields are greyed out, stop the machine first.',
  },
  storage: {
    lead: 'Running low on disk space inside a machine? The Storage panel grows a virtual disk without opening VirtualBox.',
    resize: {
      title: 'Resize a disk',
      body: 'Each disk shows its format and current size. Enter a larger size in gigabytes and press Resize — TabVM grows the virtual disk in place.',
    },
    add: {
      title: 'Add a disk',
      body: 'Need a second disk? Enter a size and press Add disk. TabVM creates a new dynamically-allocated VDI and attaches it to a free SATA port, growing the controller if it is full.',
    },
    remove: {
      title: 'Detach or delete a disk',
      body: 'Detach removes a disk from the VM but keeps its file, so you can re-attach it later — fully reversible. Delete detaches it and permanently erases the disk image from your computer; there is no undo, so both actions ask you to confirm.',
    },
    limits: {
      title: 'What can be resized',
      body: 'Only VDI and VHD disks that are dynamically allocated and have no snapshots can grow. Disks can only get bigger, never smaller, and the machine must be powered off. When a disk cannot be resized, the panel tells you why.',
    },
    tip: 'Both resizing and adding a disk only touch the virtual hardware. A new disk arrives blank — partition and format it inside the guest before use. A resized disk keeps its old partition until you expand it (for example with GParted, or growpart + resize2fs on Linux).',
  },
  files: {
    lead: 'Move files between your computer (the host) and a machine (the guest) two ways.',
    share: {
      title: 'Shared folders',
      body: 'Pick a host folder and it appears inside the guest at /media/sf_<name>. Add it while the machine is stopped to keep it permanently; add it while running for the current session.',
    },
    drop: {
      title: 'Drag & drop',
      body: 'Drop files straight onto a machine or its console. If a shared folder exists they land there; otherwise they are copied in over guest control with a one-time username and password.',
    },
    tip: 'Add the shared folder while the machine is powered OFF and it stays permanent — it survives a full shutdown and comes back every boot. Add it while running and it only lasts for that session.',
  },
  snapshots: {
    lead: 'A snapshot is a saved point in time. Take one before you experiment, then roll back if anything breaks.',
    take: {
      title: 'Take a snapshot',
      body: 'Give it a name (or accept the timestamp) and TabVM captures the whole machine state in seconds.',
    },
    restore: {
      title: 'Restore',
      body: 'Restoring powers the machine off and rewinds it to that snapshot. Anything not captured since is lost — that is the point.',
    },
    tip: 'Take a snapshot right before installing something risky or editing system files. If it breaks, one restore rewinds the whole machine — far faster than rebuilding it.',
  },
  network: {
    lead: 'Change how a machine reaches the network, per adapter, without opening VirtualBox.',
    modes: [
      { title: 'NAT', body: 'The default. The machine shares your computer’s connection and can reach the internet, but is not directly reachable from outside.' },
      { title: 'Bridged', body: 'The machine gets its own address on your real network, as if it were another device plugged into it.' },
      { title: 'Host-only', body: 'A private network between your computer and the machine only — isolated from the internet. Ideal for safe lab work.' },
    ],
    forwarding: {
      title: 'Port forwarding (NAT)',
      body: 'When an adapter is in NAT mode, you can map a port on your computer to a port inside the machine — for example forward 127.0.0.1:2222 to the guest’s port 22 to SSH in. Give each rule a name, pick TCP or UDP, and set the host and guest ports; the host address defaults to 127.0.0.1 so the port is never exposed to your whole network by accident. Rules apply live on a running machine or are saved to its config when stopped, and each is removed with one click.',
    },
    tip: 'Use Host-only when you are testing malware or an untrusted image — the guest can talk to your computer but never reaches the internet.',
  },
  create: {
    lead: 'The New VM wizard has three paths. The first two leave you with Guest Additions already working; the third gives you full control for any other OS.',
    importCard: {
      title: 'Import an image',
      body: 'Point it at a prebuilt .ova appliance (for example a Kali lab image). One click imports it, ready to start — Guest Additions already inside.',
    },
    installCard: {
      title: 'Install from ISO',
      body: 'Choose an Ubuntu, Debian or Windows ISO, set the size and account, and TabVM runs the whole install automatically with Guest Additions included.',
    },
    manualCard: {
      title: 'Other OS (manual install)',
      body: 'Pick any bootable ISO and TabVM creates the machine with the installer attached — you install the OS yourself in the console. Guest Additions are not installed automatically; use the "Install Guest Additions" button afterwards.',
    },
    cloneCard: {
      title: 'Clone an existing machine',
      body: 'A stopped machine has a Clone button. Give the copy a name and pick Full (an independent copy of the disks) or Linked (faster and smaller, but it shares the source disk and needs the source to have a snapshot). The clone runs in the background — a full clone copies the disks and can take a few minutes.',
    },
    exportCard: {
      title: 'Export a machine to an .ova',
      body: 'A stopped machine has an Export button. Pick a destination folder and TabVM writes a portable .ova appliance named after the machine — copying the disks runs in the background and can take a few minutes. Share the file or import it back on another host.',
    },
    note: 'Kali is offered through import only — VirtualBox has no automated-install template for it.',
  },
  personalize: {
    lead: 'TabVM adapts to you. These controls live in the top bar and apply everywhere, instantly.',
    theme: { title: 'Light or dark', body: 'The moon toggles the theme. Your choice is remembered and applied before the app even paints, so there is no flash.' },
    lang: { title: 'English or Spanish', body: 'The EN / ES switch changes every label, message and error — including this manual.' },
    accent: { title: 'Accent color', body: 'The paintbrush picks the accent used across the whole app. The brush itself carries the color you choose.' },
    tryHint: 'Go ahead — the controls below are live. Change the theme, language or accent and watch this page follow.',
  },
  troubleshoot: {
    lead: 'The most common questions, answered.',
    gaManual: {
      title: 'Install Guest Additions manually',
      intro: 'If the automatic install does not finish on a Linux guest, install Guest Additions by hand. In TabVM click "Install Guest Additions" first — that mounts the disc — then run these in the guest terminal:',
      commands: [
        'sudo mount /dev/cdrom /mnt',
        'sudo apt install -y build-essential dkms linux-headers-$(uname -r)',
        'sudo sh /mnt/VBoxLinuxAdditions.run',
        'sudo reboot',
      ],
      after: 'After the reboot Guest Additions is active: shared folders auto-mount, the console resizes to fit, and the clipboard can be shared.',
    },
    faqs: [
      { q: 'The dashboard says the agent is offline.', a: 'The TabVM agent is not running. Launch TabVM again; the dashboard reconnects on its own within a few seconds.' },
      { q: 'My machines do not appear.', a: 'TabVM needs Oracle VirtualBox installed. Check the Agent page under System — it reports whether VBoxManage was found.' },
      { q: 'The console screen is frozen.', a: 'The machine may still be booting, or Guest Additions is not active yet. Give it a moment; if it persists, reset the machine and reopen the console.' },
      { q: 'A shared folder is not visible in the guest.', a: 'Guest Additions must be running inside the machine, and your guest user must be in the vboxsf group. Reboot the guest after adding the folder.' },
      { q: 'The Windows install stops asking for a product key.', a: 'Some Windows ISOs require an edition or key that automated install cannot supply. Pick an ISO that installs without one, or enter it in the guest when prompted.' },
    ],
  },
  credits: {
    by: 'crafted by',
    name: 'Carlos Araúz',
  },
};

const es: DocsStrings = {
  // The slogan is part of the brand and is never translated.
  tagline: 'every VM. one tab.',
  sectionNav: 'En esta página',
  tipLabel: 'Consejo',
  sections: {
    welcome: 'Bienvenida',
    quickStart: 'Inicio rápido',
    operate: 'Operar una VM',
    terminal: 'Terminal serial',
    hardware: 'CPU y memoria',
    storage: 'Almacenamiento',
    files: 'Archivos y compartir',
    snapshots: 'Instantáneas',
    network: 'Red',
    create: 'Crear una VM',
    personalize: 'Personalizar',
    troubleshoot: 'Solución de problemas',
  },
  welcome: {
    lead: 'TabVM convierte tus máquinas locales de VirtualBox en pestañas del navegador. No instalas nada más que VirtualBox: todo lo demás corre en tu propia computadora, desde esta ventana en 127.0.0.1.',
    cards: [
      { title: 'Local y privado', body: 'El agente se enlaza solo a 127.0.0.1. Tus VMs nunca salen de tu equipo y nada se envía a la nube.' },
      { title: 'Todo en una vista', body: 'Inicia, detén, mira la pantalla, comparte archivos, crea instantáneas y cambia la red, sin abrir nunca la ventana de VirtualBox.' },
      { title: 'Pensado para aprender', body: 'Importa un appliance en un clic, gobiérnalo desde una consola guiada y sigue un manual que te muestra exactamente qué hace cada control.' },
    ],
    tryHint: 'Este manual es interactivo. Los botones que ves en las demos son los controles reales de TabVM: haz clic en ellos.',
  },
  quickStart: {
    lead: 'Tres pasos desde una instalación limpia hasta una máquina en marcha.',
    steps: [
      { title: 'Instala y abre', body: 'Ejecuta el instalador de TabVM y ábrelo. Tu navegador abre el panel en 127.0.0.1: sin inicio de sesión, sin configuración.' },
      { title: 'Agrega una máquina', body: 'Haz clic en Nueva VM. Importa un appliance listo (lo más rápido) o instala un sistema desde una ISO con Guest Additions incluido.' },
      { title: 'Inicia y conéctate', body: 'Pulsa iniciar en la máquina y luego Abrir consola para ver y controlar su pantalla dentro de la pestaña.' },
    ],
  },
  operate: {
    lead: 'Cada máquina de la lista tiene un conjunto compacto de controles. Reflejan exactamente lo que usarás a diario.',
    startStop: {
      title: 'Iniciar, detener, reiniciar',
      body: 'Una máquina detenida muestra iniciar. Una en marcha muestra detener (apagado limpio, como pulsar el botón de encendido) y reiniciar (reinicio forzado: úsalo solo si el guest está trabado).',
    },
    console: {
      title: 'La consola en vivo',
      body: 'Abrir consola se conecta a la pantalla de la máquina por VirtualBox y la transmite aquí. Haz clic dentro para enviar teclado y ratón; ábrela a pantalla completa en una pestaña nueva cuando necesites espacio.',
    },
    tip: 'Seleccionar una máquina —en marcha o apagada— abre su panel de enfoque abajo, donde viven Archivos, Red e Instantáneas.',
  },
  terminal: {
    lead: 'Abre una shell de texto de una máquina Linux en una pestaña, sin ventana gráfica. El botón aparece solo en máquinas Linux, y funciona por el puerto serial de la máquina, así que no interviene la red ni el firewall.',
    enable: {
      title: 'Actívala',
      body: 'La primera vez, apaga la máquina y haz clic en Habilitar terminal serial: eso conecta el puerto serial. Luego enciende la máquina.',
    },
    open: {
      title: 'Ábrela',
      body: 'Haz clic en el botón de terminal en la fila de la máquina. Se abre a pantalla completa en una pestaña nueva, igual que la consola.',
    },
    activate: {
      title: 'Si la pantalla queda en blanco',
      body: 'Una terminal en blanco significa que nadie atiende el puerto todavía. Una tarjeta te pide una sola vez una cuenta de la máquina virtual (root, o un usuario con sudo) y activa el inicio de sesión: entonces aparece el prompt.',
    },
    tip: 'La terminal serial nunca usa la red, así que ni el firewall ni las políticas pueden bloquearla — útil en máquinas restringidas.',
  },
  hardware: {
    lead: 'Dale a una máquina más (o menos) potencia. El panel Hardware en la vista de enfoque de una máquina edita sus CPU virtuales y su memoria, sin abrir VirtualBox.',
    edit: {
      title: 'Cambiar vCPU y memoria',
      body: 'Define el número de CPU virtuales y la memoria en megabytes, y pulsa Aplicar. El cambio se escribe directamente en la configuración de la máquina.',
    },
    limits: {
      title: 'Limitado por tu anfitrión',
      body: 'Los campos nunca te dejan asignar más procesadores o memoria de los que tu computadora realmente tiene, así no puedes dejar una máquina sin poder arrancar.',
    },
    tip: 'El hardware solo se puede cambiar con la máquina APAGADA: VirtualBox no redimensiona una VM en marcha. Si los campos están en gris, detén primero la máquina.',
  },
  storage: {
    lead: '¿Te quedas sin espacio dentro de una máquina? El panel Almacenamiento agranda un disco virtual sin abrir VirtualBox.',
    resize: {
      title: 'Redimensionar un disco',
      body: 'Cada disco muestra su formato y tamaño actual. Ingresa un tamaño mayor en gigabytes y pulsa Redimensionar: TabVM agranda el disco virtual en el lugar.',
    },
    add: {
      title: 'Agregar un disco',
      body: '¿Necesitas un segundo disco? Ingresa un tamaño y pulsa Agregar disco. TabVM crea un nuevo VDI de asignación dinámica y lo conecta a un puerto SATA libre, ampliando el controlador si está lleno.',
    },
    remove: {
      title: 'Desconectar o eliminar un disco',
      body: 'Desconectar quita un disco de la VM pero conserva su archivo, así puedes volver a conectarlo más tarde: es totalmente reversible. Eliminar lo desconecta y borra de forma permanente la imagen del disco de tu computadora; no hay deshacer, por eso ambas acciones piden confirmación.',
    },
    limits: {
      title: 'Qué se puede redimensionar',
      body: 'Solo los discos VDI y VHD de asignación dinámica y sin instantáneas pueden crecer. Los discos solo pueden agrandarse, nunca reducirse, y la máquina debe estar apagada. Cuando un disco no se puede redimensionar, el panel te dice por qué.',
    },
    tip: 'Tanto redimensionar como agregar un disco solo tocan el hardware virtual. Un disco nuevo llega en blanco: particiónalo y formatéalo dentro del guest antes de usarlo. Un disco redimensionado conserva su partición anterior hasta que la expandas (por ejemplo con GParted, o growpart + resize2fs en Linux).',
  },
  files: {
    lead: 'Mueve archivos entre tu computadora (el anfitrión) y una máquina (el guest) de dos maneras.',
    share: {
      title: 'Carpetas compartidas',
      body: 'Elige una carpeta del anfitrión y aparece dentro del guest en /media/sf_<nombre>. Agrégala con la máquina detenida para que sea permanente; agrégala en marcha para la sesión actual.',
    },
    drop: {
      title: 'Arrastrar y soltar',
      body: 'Suelta archivos directamente sobre una máquina o su consola. Si hay una carpeta compartida, caen allí; si no, se copian por guest control con un usuario y contraseña de un solo uso.',
    },
    tip: 'Agrega la carpeta compartida con la máquina APAGADA y queda permanente: sobrevive a un apagado total y vuelve en cada arranque. Si la agregas en marcha, solo dura esa sesión.',
  },
  snapshots: {
    lead: 'Una instantánea es un punto guardado en el tiempo. Crea una antes de experimentar y vuelve atrás si algo se rompe.',
    take: {
      title: 'Crear una instantánea',
      body: 'Dale un nombre (o acepta la marca de tiempo) y TabVM captura todo el estado de la máquina en segundos.',
    },
    restore: {
      title: 'Restaurar',
      body: 'Restaurar apaga la máquina y la revierte a esa instantánea. Se pierde todo lo no capturado desde entonces: de eso se trata.',
    },
    tip: 'Crea una instantánea justo antes de instalar algo arriesgado o editar archivos del sistema. Si se rompe, una sola restauración revierte toda la máquina, mucho más rápido que reconstruirla.',
  },
  network: {
    lead: 'Cambia cómo una máquina llega a la red, por adaptador, sin abrir VirtualBox.',
    modes: [
      { title: 'NAT', body: 'El modo por defecto. La máquina comparte la conexión de tu computadora y llega a internet, pero no es accesible directamente desde afuera.' },
      { title: 'Puente', body: 'La máquina obtiene su propia dirección en tu red real, como si fuera otro dispositivo conectado a ella.' },
      { title: 'Solo anfitrión', body: 'Una red privada solo entre tu computadora y la máquina, aislada de internet. Ideal para laboratorio seguro.' },
    ],
    forwarding: {
      title: 'Redirección de puertos (NAT)',
      body: 'Cuando un adaptador está en modo NAT, puedes asignar un puerto de tu computadora a un puerto dentro de la máquina; por ejemplo, redirigir 127.0.0.1:2222 al puerto 22 del guest para entrar por SSH. Dale un nombre a cada regla, elige TCP o UDP y define los puertos del anfitrión y del guest; la dirección del anfitrión usa 127.0.0.1 de forma predeterminada, así el puerto nunca queda expuesto a toda tu red por accidente. Las reglas se aplican en vivo en una máquina en ejecución o se guardan en su configuración cuando está detenida, y cada una se quita con un clic.',
    },
    tip: 'Usa Solo anfitrión cuando pruebas malware o una imagen que no es de confianza: el guest habla con tu computadora pero nunca llega a internet.',
  },
  create: {
    lead: 'El asistente Nueva VM tiene tres caminos. Los dos primeros te dejan con Guest Additions ya funcionando; el tercero te da control total para cualquier otro sistema operativo.',
    importCard: {
      title: 'Importar una imagen',
      body: 'Apúntalo a un appliance .ova prehorneado (por ejemplo una imagen de laboratorio Kali). Un clic la importa, lista para iniciar, con Guest Additions ya dentro.',
    },
    installCard: {
      title: 'Instalar desde ISO',
      body: 'Elige una ISO de Ubuntu, Debian o Windows, define el tamaño y la cuenta, y TabVM ejecuta toda la instalación automáticamente con Guest Additions incluido.',
    },
    manualCard: {
      title: 'Otro SO (instalación manual)',
      body: 'Elige cualquier ISO arrancable y TabVM crea la máquina con el instalador adjunto: tú instalas el sistema operativo en la consola. Guest Additions no se instala automáticamente; usa después el botón "Instalar Guest Additions".',
    },
    cloneCard: {
      title: 'Clonar una máquina existente',
      body: 'Una máquina apagada muestra un botón Clonar. Da un nombre a la copia y elige Completo (una copia independiente de los discos) o Enlazado (más rápido y ligero, pero comparte el disco de origen y requiere que el origen tenga una instantánea). El clon corre en segundo plano: un clon completo copia los discos y puede tardar unos minutos.',
    },
    exportCard: {
      title: 'Exportar una máquina a un .ova',
      body: 'Una máquina apagada muestra un botón Exportar. Elige una carpeta de destino y TabVM escribe un dispositivo .ova portátil con el nombre de la máquina: copiar los discos corre en segundo plano y puede tardar unos minutos. Comparte el archivo o vuelve a importarlo en otro host.',
    },
    note: 'Kali se ofrece solo por importación: VirtualBox no tiene plantilla de instalación automatizada para él.',
  },
  personalize: {
    lead: 'TabVM se adapta a ti. Estos controles viven en la barra superior y se aplican en todas partes, al instante.',
    theme: { title: 'Claro u oscuro', body: 'La luna alterna el tema. Tu elección se recuerda y se aplica antes de que la app pinte, así no hay parpadeo.' },
    lang: { title: 'Inglés o español', body: 'El interruptor EN / ES cambia cada etiqueta, mensaje y error, incluido este manual.' },
    accent: { title: 'Color de acento', body: 'El pincel elige el acento usado en toda la app. El pincel mismo lleva el color que elijas.' },
    tryHint: 'Adelante: los controles de abajo están vivos. Cambia el tema, el idioma o el acento y mira cómo esta página los sigue.',
  },
  troubleshoot: {
    lead: 'Las preguntas más comunes, respondidas.',
    gaManual: {
      title: 'Instalar Guest Additions manualmente',
      intro: 'Si la instalación automática no termina en un guest Linux, instala Guest Additions a mano. En TabVM haz clic primero en "Instalar Guest Additions" —eso monta el disco— y luego ejecuta esto en la terminal del guest:',
      commands: [
        'sudo mount /dev/cdrom /mnt',
        'sudo apt install -y build-essential dkms linux-headers-$(uname -r)',
        'sudo sh /mnt/VBoxLinuxAdditions.run',
        'sudo reboot',
      ],
      after: 'Tras el reinicio, Guest Additions queda activo: las carpetas compartidas se montan solas, la consola se ajusta al tamaño y se puede compartir el portapapeles.',
    },
    faqs: [
      { q: 'El panel dice que el agente está desconectado.', a: 'El agente de TabVM no está corriendo. Abre TabVM de nuevo; el panel se reconecta solo en unos segundos.' },
      { q: 'Mis máquinas no aparecen.', a: 'TabVM necesita Oracle VirtualBox instalado. Revisa la página Agente en Sistema: indica si se encontró VBoxManage.' },
      { q: 'La pantalla de la consola está congelada.', a: 'La máquina puede seguir arrancando, o Guest Additions aún no está activo. Dale un momento; si persiste, reinicia la máquina y reabre la consola.' },
      { q: 'Una carpeta compartida no se ve en el guest.', a: 'Guest Additions debe estar corriendo dentro de la máquina, y tu usuario del guest debe estar en el grupo vboxsf. Reinicia el guest tras agregar la carpeta.' },
      { q: 'La instalación de Windows se detiene pidiendo una clave de producto.', a: 'Algunas ISOs de Windows requieren una edición o clave que la instalación automatizada no puede aportar. Usa una ISO que instale sin ella, o ingrésala en el guest cuando la pida.' },
    ],
  },
  credits: {
    by: 'hecho por',
    name: 'Carlos Araúz',
  },
};

const docsContent: Record<Lang, DocsStrings> = { en, es };

// useDocs returns the docs content for the active language.
export function useDocs(): DocsStrings {
  const { lang } = useLang();
  return docsContent[lang];
}
