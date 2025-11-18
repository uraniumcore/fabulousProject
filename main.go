package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// Полная серверная модель (с правильным ответом)
type Question struct {
	ID       int      `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Answer   int      `json:"answer"` // индекс правильного варианта
}

// Публичная модель для фронта (без правильного ответа)
type PublicQuestion struct {
	ID       int      `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

type StartRequest struct {
	User string `json:"user"`
}

type StartResponse struct {
	Success   bool             `json:"success"`
	TestID    string           `json:"test_id"`
	Questions []PublicQuestion `json:"test"`
}

// Запрос с ответами пользователя
type SubmitRequest struct {
	TestID  string `json:"test_id"`
	User    string `json:"user"`
	Answers []struct {
		QuestionID int `json:"question_id"`
		Choice     int `json:"choice"`
	} `json:"answers"`
}

// Ответ с баллом и подробным разбором
type SubmitResponse struct {
	Success bool         `json:"success"`
	Score   int          `json:"score"`
	Total   int          `json:"total"`
	Results []ReviewItem `json:"results"`
}

type ReviewItem struct {
	QuestionID    int      `json:"question_id"`
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectChoice int      `json:"correct_choice"`
	UserChoice    int      `json:"user_choice"`
}

// Хранилище попыток по test_id
type TestStore struct {
	mu        sync.RWMutex
	testMap   map[string][]Question // test_id -> полный список вопросов с ответами
	expiresAt map[string]time.Time  // test_id -> время истечения (необязательно)
	ttl       time.Duration
}

func NewTestStore(ttl time.Duration) *TestStore {
	return &TestStore{
		testMap:   make(map[string][]Question),
		expiresAt: make(map[string]time.Time),
		ttl:       ttl,
	}
}

func (s *TestStore) Put(testID string, qs []Question) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.testMap[testID] = qs
	if s.ttl > 0 {
		s.expiresAt[testID] = time.Now().Add(s.ttl)
	}
}

func (s *TestStore) Get(testID string) ([]Question, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	qs, ok := s.testMap[testID]
	if !ok {
		return nil, false
	}
	if s.ttl > 0 {
		if exp, ok2 := s.expiresAt[testID]; ok2 && time.Now().After(exp) {
			return nil, false
		}
	}
	return qs, true
}

func (s *TestStore) CleanupExpired() {
	if s.ttl == 0 {
		return
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, exp := range s.expiresAt {
		if now.After(exp) {
			delete(s.testMap, id)
			delete(s.expiresAt, id)
		}
	}
}

// Пример — “база” вопросов в памяти
var baseQuestions = []Question{
	{
		ID:       1,
		Question: "An employee reports that each time a workstation is started it locks up after about 5 minutes of use. What is the most likely cause of the problem?",
		Options: []string{
			"The CPU is overheating.",
			"The hard disk is failing.",
			"The RAM is malfunctioning.",
			"The power supply fails to provide adequate voltage and current.",
		},
		Answer: 0, // CPU overheating
	},
	{
		ID:       2,
		Question: "A customer has a computer for a home business, but wants to have another computer as a web server. What would be the best solution for the customer to share the monitor, mouse, and keyboard between the two computers?",
		Options: []string{
			"Multipurpose device",
			"KVM switch",
			"Access point",
			"USB hub",
		},
		Answer: 1, // KVM switch
	},
	{
		ID:       3,
		Question: "If you do not want to use static IP addresses, but want to dynamically assign IP addresses to computers instead, what network protocol can you use?",
		Options: []string{
			"DHCP",
			"NTP",
			"Kerberos",
			"TFTP",
		},
		Answer: 0, // DHCP
	},
	{
		ID:       4,
		Question: "How is a power supply commonly attached to a tower case?",
		Options: []string{
			"Screws",
			"Retention bar",
			"Standoffs",
			"Restraining strap",
		},
		Answer: 0, // Screws
	},
	{
		ID:       5,
		Question: "Which component is measured in GHz?",
		Options: []string{
			"Hard disk",
			"Processor",
			"RAM",
			"Power supply",
		},
		Answer: 1, // Processor
	},
	{
		ID:       6,
		Question: "Adapter cards slide into ____.",
		Options: []string{
			"Power slots",
			"PCIe slots",
			"Processor slots",
			"Memory slots",
		},
		Answer: 1, // PCIe slots
	},
	{
		ID:       7,
		Question: "What is a characteristic of a WAN?",
		Options: []string{
			"It connects multiple networks that are geographically separated.",
			"It spans across a campus or city to enable sharing of regional resources.",
			"It requires a wireless access point to connect users to the network.",
			"It is typically owned and managed by a single home or business.",
		},
		Answer: 0, // WAN connects geographically separated networks
	},
	{
		ID:       8,
		Question: "Which is NOT the best example of a peripheral?",
		Options: []string{
			"Printer",
			"Keyboard",
			"Speakers",
			"Hard disk",
		},
		Answer: 3, // Hard disk (обычно внутренняя)
	},
	{
		ID:       9,
		Question: "What electrical unit refers to the number of electrons moving through a circuit per second?",
		Options: []string{
			"Voltage",
			"Resistance",
			"Power",
			"Current",
		},
		Answer: 3, // Current
	},
	{
		ID:       10,
		Question: "Which characteristic describes EPROM?",
		Options: []string{
			"Chips specifically designed for video graphics that are used in conjunction with a dedicated GPU",
			"Chips that require constant power to function and are often used for cache memory",
			"A chip that is nonvolatile and can be erased by exposing it to strong ultraviolet light",
			"Chips that run at clock speeds of 800 MHz and have a connector with 240 pins",
		},
		Answer: 2, // UV erasable non‑volatile
	},
	{
		ID:       11,
		Question: "Which smart home wireless technology has an open standard that allows up to 232 devices to be connected?",
		Options: []string{
			"802.11n",
			"802.11ac",
			"Zigbee",
			"Z‑Wave",
		},
		Answer: 3, // Z‑Wave (до 232 устройств)
	},
	{
		ID:       12,
		Question: "When a new PC is being built, which component has the most influence when selecting the case and power supply?",
		Options: []string{
			"RAM module",
			"Video card",
			"Hard disk type",
			"Motherboard",
			"Sound card",
		},
		Answer: 3, // Motherboard
	},
	{
		ID:       13,
		Question: "Which action can reduce the risk of ESD damage when computer equipment is being worked on?",
		Options: []string{
			"Keeping the computer plugged into a surge protector",
			"Lowering the humidity level in the work area",
			"Working on a grounded antistatic mat",
			"Moving cordless phones away from the work area",
		},
		Answer: 2, // Grounded antistatic mat
	},
	{
		ID:       14,
		Question: "What is an active cooling solution for a PC?",
		Options: []string{
			"Reduce the speed of the CPU.",
			"Add a heatsink to the CPU.",
			"Use a painted computer case.",
			"Add an additional case fan.",
		},
		Answer: 3, // Additional case fan (active)
	},
	{
		ID:       15,
		Question: "How are the internal components of a computer protected against ESD?",
		Options: []string{
			"By unplugging the computer after use",
			"By using multiple fans to move warm air through the case",
			"By using computer cases made out of plastic or aluminum",
			"By grounding the internal components via attachment to the case",
		},
		Answer: 3, // Grounding via case
	},
	{
		ID:       16,
		Question: "A technician is searching through a storage locker and finds a firewall. What is the purpose of this device?",
		Options: []string{
			"It is a device that can be inserted in the middle of a cable run to add power.",
			"It is a device that uses existing electrical wiring to connect devices and sends data using specific frequencies.",
			"It connects a home or business network to a company that provides internet connectivity as well as television signals.",
			"It is placed between two or more networks and protects data and equipment from unauthorized access.",
		},
		Answer: 3, // Firewall protects between networks
	},
	{
		ID:       17,
		Question: "Which pairs of wires change termination order between the 568A and 568B standards?",
		Options: []string{
			"Green and brown",
			"Green and orange",
			"Brown and orange",
			"Blue and brown",
		},
		Answer: 1, // Green and orange
	},
	{
		ID:       18,
		Question: "Which type of interface was originally developed for high-definition televisions and is also popular to use with computers to connect audio and video devices?",
		Options: []string{
			"HDMI",
			"FireWire",
			"USB",
			"DVI",
			"VGA",
		},
		Answer: 0, // HDMI
	},
	{
		ID:       19,
		Question: "An example of something that operates at the application layer is:",
		Options: []string{
			"TCP",
			"A web browser",
			"A router",
			"UDP",
		},
		Answer: 1, // Web browser
	},
	{
		ID:       20,
		Question: "What does the “A” in P‑A‑S‑S remind a person to do while using a fire extinguisher?",
		Options: []string{
			"Aim the fire extinguisher at the base of the fire.",
			"Activate the fire extinguisher.",
			"Aim the fire extinguisher at the flames.",
			"Adjust the pressure.",
		},
		Answer: 0, // Aim at the base
	},
	{
		ID:       21,
		Question: "Which subject area describes collecting and analyzing data from computer systems, networks, and storage devices as part of an investigation of alleged illegal activity?",
		Options: []string{
			"Cryptography",
			"Disaster recovery",
			"Computer forensics",
			"Cyber law",
		},
		Answer: 2, // Computer forensics
	},
	{
		ID:       22,
		Question: "A local service shop needs to replace one of its printers. The current printer is used to print invoices on 3‑part carbonless forms with continuous feed. Which type of printer would meet the requirement?",
		Options: []string{
			"Inkjet",
			"Laser",
			"Thermal",
			"Dot matrix",
		},
		Answer: 3, // Dot matrix
	},
	{
		ID:       23,
		Question: "A technician wishes to deploy Windows 10 Pro to multiple PCs through the remote network installation process. The technician begins by connecting the new PCs to the network and booting them up. However, the deployment fails because the target PCs are unable to communicate with the deployment server. What is the possible cause?",
		Options: []string{
			"The NIC cards on the new PCs are not PXE‑enabled.",
			"The wrong network drivers are loaded in the image file.",
			"The SID has not been changed in the image file.",
			"Sysprep was not used before building the image file.",
		},
		Answer: 0, // NIC not PXE-enabled
	},
	{
		ID:       24,
		Question: "A third‑party security firm is performing a security audit of a company and recommends the company utilize the Remote Desktop Protocol. What are two characteristics of the Microsoft Remote Desktop Protocol (RDP)? (Choose two.)",
		Options: []string{
			"RDP connects on TCP port 22.",
			"RDP connects on TCP port 3389.",
			"RDP is a command‑line network virtual terminal protocol.",
			"RDP requires a Windows client.",
			"RDP uses an encrypted session.",
		},
		// В твоей текущей модели один Answer.
		// Если хочешь строго multiple‑choice (две галочки),
		// можно хранить []int или сделать два отдельных вопроса.
		Answer: 1, // если оставлять один — порт 3389
	},
	{
		ID:       25,
		Question: "Which two actions should a technician take if illegal content is discovered on the hard drive of a customer computer? (Choose two.)",
		Options: []string{
			"Document as much information as possible.",
			"Contact a first responder.",
			"Remove and destroy the hard drive.",
			"Shut down the computer until authorities arrive.",
			"Confront the customer immediately.",
		},
		Answer: 1, // аналогично, для single‑answer нужен другой формат
	},
	{
		ID:       26,
		Question: "Why is a full format more beneficial than a quick format when preparing for a clean OS installation?",
		Options: []string{
			"A full format will delete every partition on the hard drive.",
			"A full format will delete files from the disk while analyzing the disk drive for errors.",
			"A full format uses the faster FAT32 file system, whereas a quick format uses the slower NTFS file system.",
			"A full format is the only method of installing Windows 8.1 on a PC that has an operating system currently installed.",
		},
		Answer: 1, // delete files + check for errors
	},
	{
		ID:       27,
		Question: "The CIO wants to secure data on company laptops by implementing file encryption. The technician determines the best method is to encrypt each hard drive using Windows BitLocker. Which two things are needed to implement this solution? (Choose two.)",
		Options: []string{
			"Password management",
			"At least two volumes",
			"USB stick",
			"Backup",
			"TPM",
			"EFS",
		},
		Answer: 4, // опять же, это multiple‑choice; тут нужен другой тип
	},
	{
		ID:       28,
		Question: "A technician wants to allow many computers to print to the same printer, but does not want to affect the performance of the computers. What will the technician do to achieve this?",
		Options: []string{
			"Use a computer‑shared printer.",
			"Use a software print server.",
			"Use a hardware print server.",
			"Install a second printer.",
		},
		Answer: 2, // Hardware print server
	},
	{
		ID:       29,
		Question: "A technician has connected a new internal hard drive to a Windows 10 PC. What must be done in order for Windows 10 to use the new hard drive?",
		Options: []string{
			"Run chkdsk on the new hard drive.",
			"Extend the partition on an existing hard drive to the new hard drive.",
			"Mount the new hard drive.",
			"Initialize the new hard drive.",
		},
		Answer: 3, // Initialize
	},
	{
		ID:       30,
		Question: "What is used to control illegal use of software and content?",
		Options: []string{
			"End User License Agreement",
			"Digital rights management",
			"Service level agreement",
			"Chain of custody",
		},
		Answer: 1, // DRM
	},
	{
		ID:       31,
		Question: "What two actions are appropriate for a support desk technician to take when assisting customers? (Choose two.)",
		Options: []string{
			"As soon as you detect customer anger, pass the angry customer to the next level.",
			"If you have to put the customer on hold, ask the customer for permission.",
			"Interrupt customers if they start to solve their own problems.",
			"Comfort a customer by minimizing the customer problem.",
			"Let a customer finish talking before asking additional questions.",
		},
		Answer: 1, // multiple‑answer, см. замечание ниже
	},
	{
		ID:       32,
		Question: "Which condition is required when planning to install Windows on a GPT disk?",
		Options: []string{
			"Only one primary partition can contain an OS.",
			"The computer must be UEFI‑based.",
			"The maximum partition size cannot exceed 2 TB.",
			"The maximum number of primary partitions that can co‑exist is 4.",
		},
		Answer: 1, // UEFI‑based
	},
	{
		ID:       33,
		Question: "When responding to a call from a customer who is experiencing problems with a computer, the technician notices that a number of system files on the computer have been renamed. Which two possible solutions could the technician implement to resolve the problem? (Choose two.)",
		Options: []string{
			"Reset the password of the user.",
			"Restore the computer from a backup.",
			"Change the folder and file permissions of the user.",
			"Use antivirus software to remove a virus.",
			"Upgrade the file encryption protocol.",
		},
		Answer: 3, // multiple‑answer, здесь правильные b и d
	},
	{
		ID:       34,
		Question: "A user cannot open several apps on a cell phone and takes the device into a repair shop. What is one possible cause for this situation?",
		Options: []string{
			"The apps that do open are using too many resources.",
			"The apps were installed on a memory card and the card has been removed.",
			"The apps are in an area of memory storage that is corrupt and can no longer be used.",
			"The apps do not have the required root‑level permission to operate.",
		},
		Answer: 1, // Installed on removed memory card
	},
	{
		ID:       35,
		Question: "A technician uses the Microsoft Deployment Image Servicing and Management (DISM) tool to create a Windows image file on one of the workstations running Windows 10. When the technician tries to clone another workstation with the image file, the workstation exhibits network connectivity issues on completion. What could cause this?",
		Options: []string{
			"The Sysprep utility should have been turned off prior to the creation of the image file.",
			"The SID of the original PC is not cleared when creating the image with DISM.",
			"The network drivers were not added to the image file.",
			"The technician used the wrong tool to create the image file.",
		},
		Answer: 2, // Network drivers not added
	},
	{
		ID:       36,
		Question: "What skill is essential for a level one technician to have?",
		Options: []string{
			"The ability to ask the customer relevant questions, and as soon as this information is included in the work order, escalate it to the level two technician",
			"Ability to take the work order prepared by the level two technician and try to resolve the problem",
			"The ability to translate a description of a customer problem into a few succinct sentences and enter it into the work order",
			"The ability to gather relevant information from the customer and pass it to the level two technician so it can be entered into the work order",
		},
		Answer: 2, // Сжатое описание в work order
	},
	{
		ID:       37,
		Question: "Which type of security threat can be transferred through email and is used to gain sensitive information by recording the keystrokes of the email recipient?",
		Options: []string{
			"Trojan",
			"Worm",
			"Grayware",
			"Adware",
			"Virus",
		},
		Answer: 0, // Trojan (keylogger)
	},
	{
		ID:       38,
		Question: "A user is looking for a laptop with touchscreen capabilities. Which technology makes the display of a laptop a touchscreen?",
		Options: []string{
			"Digitizer",
			"OLED",
			"LED",
			"Inverter",
		},
		Answer: 0, // Digitizer
	},
	{
		ID:       39,
		Question: "What service does PRINT$ provide?",
		Options: []string{
			"It provides printer drivers for printer administrators.",
			"It provides a group of hidden printers that only administrative users have permissions to send print jobs to.",
			"It provides an administrative printer share accessible by all local user accounts.",
			"It provides a network share for accessing shared printers.",
		},
		Answer: 0, // Printer drivers share
	},
	{
		ID:       40,
		Question: "A user is installing a local laser printer and prints a test page after installation. However, the printer prints unknown characters. What is the most likely cause of the problem?",
		Options: []string{
			"The toner cartridge is low.",
			"An incorrect printer driver is installed.",
			"The cable connection is loose.",
			"The toner cartridge is defective.",
		},
		Answer: 1, // Incorrect printer driver
	},
	{
	    ID:       41,
	    Question: "A customer comes into a computer parts and service store. The customer is looking for a device to allow secure access to the main doors of the company by swiping an ID card. What device should the store owner recommend to accomplish the required task?",
	    Options: []string{
	        "joystick or gamepad",
	        "projector",
	        "AR headset",
	        "magstripe reader",
	    },
	    Answer: 3, // magstripe reader
	},
	{
	    ID:       42,
	    Question: "A customer comes into a computer parts and service store. The customer is looking for a device to display a promotional presentation to a large audience at a conference. What device should the store owner recommend to accomplish the required task?",
	    Options: []string{
	        "joystick or gamepad",
	        "magstripe reader",
	        "projector",
	        "AR headset",
	    },
	    Answer: 2, // projector
	},
	{
	    ID:       43,
	    Question: "A customer comes into a computer parts and service store. The customer is looking for a device to help a person with accessibility issues input instructions into a laptop by using a pen. What device should the store owner recommend to accomplish the required task?",
	    Options: []string{
	        "stylus",
	        "biometric scanner",
	        "keyboard",
	        "NFC device",
	    },
	    Answer: 0, // stylus
	},
	{
	    ID:       44,
	    Question: "A customer comes into a computer parts and service store. The customer is looking for a device to allow secure access to the main doors of the company by swiping an ID card. What device should the store owner recommend to accomplish the required task?",
	    Options: []string{
	        "magstripe reader",
	        "biometric scanner",
	        "keyboard",
	        "NFC device",
	    },
	    Answer: 0, // magstripe reader
	},
	{
	    ID:       45,
	    Question: "A customer comes into a computer parts and service store. The customer is looking for a device to provide secure access to the central server room using a retinal scan. What device should the store owner recommend to accomplish the required task?",
	    Options: []string{
	        "biometric scanner",
	        "keyboard",
	        "NFC device",
	        "flatbed scanner",
	    },
	    Answer: 0, // biometric scanner
	},
	{
	    ID:       46,
	    Question: "A customer comes into a computer parts and service store. The customer is looking for a device to train pilots how to land and take off in a computer simulation environment. What device should the store owner recommend to accomplish the required task?",
	    Options: []string{
	        "projector",
	        "joystick or gamepad",
	        "magstripe reader",
	        "AR headset",
	    },
	    Answer: 1, // joystick or gamepad
	},
	{
	    ID:       47,
	    Question: "A customer comes into a computer parts and service store. The customer is looking for a device to manually input text for a new networking textbook that the customer is writing. What device should the store owner recommend to accomplish the required task?",
	    Options: []string{
	        "biometric scanner",
	        "NFC device",
	        "flatbed scanner",
	        "keyboard",
	    },
	    Answer: 3, // keyboard
	},
	{
	    ID:       48,
	    Question: "A customer comes into a computer parts and service store. The customer is looking for a device to allow users to tap and pay for their purchases. What device should the store owner recommend to accomplish the required task?",
	    Options: []string{
	        "NFC device",
	        "joystick or gamepad",
	        "projector",
	        "magstripe reader",
	    },
	    Answer: 0, // NFC device
	},
	{
	    ID:       49,
	    Question: "A customer comes into a computer parts and service store. The customer is looking for a device to help when repairing an airplane and that will allow the customer to see and interact with the repair manual at the same time. What device should the store owner recommend to accomplish the required task?",
	    Options: []string{
	        "biometric scanner",
	        "keyboard",
	        "AR headset",
	        "NFC device",
	    },
	    Answer: 2, // AR headset
	},
	{
	    ID:       50,
	    Question: "A customer has a computer for a home business, but wants to have another computer as a web server. What would be the best solution for the customer to share the monitor, mouse, and keyboard between the two computers?",
	    Options: []string{
	        "access point",
	        "KVM switch",
	        "multipurpose device",
	        "USB hub",
	    },
	    Answer: 1, // KVM switch
	},
	{
	    ID:       51,
	    Question: "A customer has a computer for a home business, but wants to have another computer as a web server. What would be the best solution for the customer to share the monitor, mouse, and keyboard between the two computers?",
	    Options: []string{
	        "KVM switch",
	        "access point",
	        "multipurpose device",
	        "network switch",
	        "USB hub",
	    },
	    Answer: 0, // KVM switch
	},
	{
	    ID:       52,
	    Question: "A device that blocks traffic that meets certain criteria is know as a ________.",
	    Options: []string{
	        "Switch",
	        "Firewall",
	        "Router",
	        "Hub",
	    },
	    Answer: 1, // Firewall
	},
	{
	    ID:       53,
	    Question: "A network specialist has been hired to install a network in a company that assembles airplane engines. Because of the nature of the business, the area is highly affected by electromagnetic interference. Which type of network media should be recommended so that the data communication will not be affected by EMI?",
	    Options: []string{
	        "UTP",
	        "STP",
	        "fiber optic",
	        "coaxial",
	    },
	    Answer: 2, // fiber optic
	},
	{
	    ID:       54,
	    Question: "A proxy is something that _______________________." ,
	    Options: []string{
	        "sends data across a single network segment.",
	        "communicates on behalf of something else.",
	        "encrypts traffic sent across the Internet.",
	        "allows for many devices to speak to one other device.",
	    },
	    Answer: 1, // communicates on behalf of something else
	},
	{
	    ID:       55,
	    Question: "A student is looking to add memory in order to speed up a tower computer. Which type of memory module should the student be looking for?",
	    Options: []string{
	        "DIMM",
	        "DIP",
	        "SIMM",
	        "SODIMM",
	    },
	    Answer: 0, // DIMM
	},
	{
	    ID:       56,
	    Question: "A technician is building a thick client workstation that would be used to run a database and wants to ensure the best protection against errors. What type of memory would be best suited for this?",
	    Options: []string{
	        "RDRAM",
	        "ECC",
	        "DDR3",
	        "DDR2",
	    },
	    Answer: 1, // ECC
	},
	{
	    ID:       57,
	    Question: "A technician is installing a new high-end video adapter card into an expansion slot on a motherboard. What may be needed to operate this video adapter card?",
	    Options: []string{
	        "24-pin ATX power connector",
	        "PCI expansion slot",
	        "Two 8-pin power connectors",
	        "PCIe x 8 expansion slot",
	    },
	    Answer: 2, // Two 8-pin power connectors
	},
	{
	    ID:       58,
	    Question: "A technician is performing hardware maintenance of PCs at a construction site. What task should the technician perform as part of a preventive maintenance plan?",
	    Options: []string{
	        "Back up the data, reformat the hard drive, and reinstall the data.",
	        "Remove dust from intake fans.",
	        "Perform an audit of all software that is installed.",
	        "Develop and install forensic tracking software.",
	    },
	    Answer: 1, // Remove dust from intake fans
	},
	{
	    ID:       59,
	    Question: "A technician is searching through a storage locker and finds a firewall. What is the purpose of this device?",
	    Options: []string{
	        "It is placed between two or more networks and protects data and equipment from unauthorized access.",
	        "It is a device that uses existing electrical wiring to connect devices and sends data using specific frequencies.",
	        "It is a device that can be inserted in the middle of a cable run to add power.",
	        "It connects a home or business network to a company that provides internet connectivity as well as television signals.",
	    },
	    Answer: 0, // Network protection between networks
	},
	{
	    ID:       60,
	    Question: "A technician looks at a motherboard and sees a 24-pin connector. What component would connect to the motherboard through the use of this 24-pin connector?",
	    Options: []string{
	        "power supply",
	        "video card",
	        "PATA optical drive",
	        "SATA drive",
	        "floppy drive",
	    },
	    Answer: 0, // power supply
	},
	{
	    ID:       61,
	    Question: "A user playing a game on a gaming PC with a standard EIDE 5400 RPM hard drive finds the performance unsatisfactory. Which hard drive upgrade would improve performance while providing more reliability and more energy efficiency?",
	    Options: []string{
	        "a 7200 RPM SATA hard drive",
	        "a 7200 RPM EIDE hard drive",
	        "a 10,000 RPM SATA hard drive",
	        "an SSD",
	    },
	    Answer: 3, // SSD
	},
	{
	    ID:       62,
	    Question: "A web designer installed the latest video editing software and now notices that when the application loads, it responds slowly. Also the hard disk LED is constantly flashing when the application is in use. What is a solution to solve the performance problem?",
	    Options: []string{
	        "upgrading to a faster CPU",
	        "replacing the video card with a model that has a DVI output",
	        "adding more RAM",
	        "replacing the hard disk with a faster model",
	    },
	    Answer: 2, // adding more RAM
	},
	{
	    ID:       63,
	    Question: "Adapter cards slide into ____.",
	    Options: []string{
	        "PCIe slots",
	        "memory slots",
	        "processor slots",
	        "power slots",
	    },
	    Answer: 0, // PCIe slots
	},
	{
	    ID:       64,
	    Question: "All programs that are currently running are located in the _____.",
	    Options: []string{
	        "RAM",
	        "Hard Disk",
	        "Processor",
	        "Motherboard",
	    },
	    Answer: 0, // RAM
	},
	{
	    ID:       65,
	    Question: "An employee reports that each time a workstation is started it locks up after about 5 minutes of use. What is the most likely cause of the problem?",
	    Options: []string{
	        "The RAM is malfunctioning.",
	        "The CPU is overheating.",
	        "The power supply fails to provide adequate voltage and current.",
	        "The hard disk is failing.",
	    },
	    Answer: 1, // CPU overheating
	},
	{
	    ID:       66,
	    Question: "An example of something that operates at the application layer is:",
	    Options: []string{
	        "TCP",
	        "A web browser",
	        "UDP",
	        "A router",
	    },
	    Answer: 1, // A web browser
	},
	{
	    ID:       67,
	    Question: "How are the internal components of a computer protected against ESD?",
	    Options: []string{
	        "by using computer cases made out of plastic or aluminum",
	        "by unplugging the computer after use",
	        "by using multiple fans to move warm air through the case",
	        "by grounding the internal components via attachment to the case.",
	    },
	    Answer: 3, // grounding via case
	},
	{
	    ID:       68,
	    Question: "How is a power supply commonly attached to a tower case?",
	    Options: []string{
	        "standoffs",
	        "screws",
	        "restraining strap",
	        "retention bar",
	    },
	    Answer: 1, // screws
	},
	{
	    ID:       69,
	    Question: "If you don't want to use static IP addresses, but want to dynamically assign IP addresses to computers instead, what network protocol can you use?",
	    Options: []string{
	        "TFTP",
	        "Kerberos",
	        "NTP",
	        "DHCP",
	    },
	    Answer: 3, // DHCP
	},
	{
	    ID:       70,
	    Question: "VPNs are known as a _____ protocol.",
	    Options: []string{
	        "connectionless",
	        "data link layer",
	        "network layer",
	        "tunneling",
	    },
	    Answer: 3, // tunneling
	},
	{
	    ID:       71,
	    Question: "What component is most suspect if a burning electronics smell is evident?",
	    Options: []string{
	        "RAM module",
	        "CPU",
	        "hard drive",
	        "power supply",
	    },
	    Answer: 3, // power supply
	},
	{
	    ID:       72,
	    Question: "What data is stored in the CMOS memory chip?",
	    Options: []string{
	        "user login information",
	        "device drivers",
	        "Windows configuration settings",
	        "BIOS settings",
	    },
	    Answer: 3, // BIOS settings
	},
	{
	    ID:       73,
	    Question: "What does LAN stand for?",
	    Options: []string{
	        "Large area network",
	        "Little area network",
	        "Local area network",
	        "Locally available network",
	    },
	    Answer: 2, // Local area network
	},
	{
	    ID:       74,
	    Question: "What does the “A” in P-A-S-S remind a person to do while using a fire extinguisher?",
	    Options: []string{
	        "Adjust the pressure.",
	        "Aim the fire extinguisher at the flames.",
	        "Aim the fire extinguisher at the base of the fire.",
	        "Activate the fire extinguisher.",
	    },
	    Answer: 2, // Aim at base
	},
	{
	    ID:       75,
	    Question: "What does WAN stand for?",
	    Options: []string{
	        "Wireless Local Area Network",
	        "Wireless Area Network",
	        "Wired Area Network",
	        "Wide Area Network",
	    },
	    Answer: 3, // Wide Area Network
	},
	{
	    ID:       76,
	    Question: "What electrical unit refers to the number of electrons moving through a circuit per second?",
	    Options: []string{
	        "resistance",
	        "current",
	        "voltage",
	        "power",
	    },
	    Answer: 1, // current
	},
	{
	    ID:       77,
	    Question: "What is a characteristic of a WAN?",
	    Options: []string{
	        "It connects multiple networks that are geographically separated.",
	        "It spans across a campus or city to enable sharing of regional resources.",
	        "It is typically owned and managed by a single home or business.",
	        "It requires a wireless access point to connect users to the network.",
	    },
	    Answer: 0, // geographically separated networks
	},
	{
	    ID:       78,
	    Question: "What is a primary benefit of preventive maintenance on a PC?",
	    Options: []string{
	        "It simplifies PC use for the end user.",
	        "It extends the life of the components.",
	        "It assists the user in software development.",
	        "It enhances the troubleshooting processes.",
	    },
	    Answer: 1, // extends life
	},
	{
	    ID:       79,
	    Question: "What is an active cooling solution for a PC?",
	    Options: []string{
	        "Reduce the speed of the CPU.",
	        "Add a heatsink to the CPU.",
	        "Add an additional case fan.",
	        "Use a painted computer case.",
	    },
	    Answer: 2, // additional case fan
	},
	{
	    ID:       80,
	    Question: "What is an active cooling solution for a PC?",
	    Options: []string{
	        "Add a heatsink to the CPU.",
	        "Reduce the speed of the CPU.",
	        "Use a painted computer case.",
	        "Add an additional case fan.",
	    },
	    Answer: 3, // additional case fan
	},
	{
	    ID:       81,
	    Question: "What is one purpose of adjusting the clock speed within the BIOS configuration settings?",
	    Options: []string{
	        "to disable devices that are not needed or used by the computer",
	        "to change the order of the bootable partitions",
	        "to allow a computer to run multiple operating systems in files or partitions",
	        "to allow the computer to run slower and cooler",
	    },
	    Answer: 3, // run slower and cooler
	},
	{
	    ID:       82,
	    Question: "What is used to prevent the motherboard from touching metal portions of the computer case?",
	    Options: []string{
	        "an I/O shield",
	        "standoffs",
	        "thermal compound",
	        "ZIF sockets",
	    },
	    Answer: 1, // standoffs
	},
	{
	    ID:       83,
	    Question: "What transport layer protocol does DNS normally use?",
	    Options: []string{
	        "IP",
	        "TCP",
	        "ICMP",
	        "UDP",
	    },
	    Answer: 3, // UDP
	},
	{
	    ID:       84,
	    Question: "What's a router?",
	    Options: []string{
	        "more advanced version of a switch",
	        "A network device used specially for fiber cables",
	        "A device that knows how to forward data between independent networks",
	        "A physical layer device that prevents crosstalk",
	    },
	    Answer: 2, // forwards between networks
	},
	{
	    ID:       85,
	    Question: "What's the difference between a client and a server?",
	    Options: []string{
	        "Clients operate on the data link layer, and servers operate on the network layer.",
	        "A client requests data, and a server responds to that request.",
	        "Clients and servers are different names for the same thing.",
	        "A server requests data, and a client responds to that request.",
	    },
	    Answer: 1, // client requests, server responds
	},
	{
	    ID:       86,
	    Question: "When a new PC is being built, which component has the most influence when selecting the case and power supply?",
	    Options: []string{
	        "hard disk type",
	        "motherboard",
	        "RAM module",
	        "sound card",
	        "video card",
	    },
	    Answer: 1, // motherboard
	},
	{
	    ID:       87,
	    Question: "When you have a web server, what service is used to enable HTTP requests to be processed?",
	    Options: []string{
	        "An HTTP server",
	        "The web server",
	        "HTTP status codes",
	        "A database server",
	    },
	    Answer: 0, // HTTP server
	},
	{
	    ID:       88,
	    Question: "Where is buffered memory commonly used?",
	    Options: []string{
	        "servers",
	        "gaming computers",
	        "gaming laptops",
	        "business PCs",
	        "tablets",
	    },
	    Answer: 0, // servers
	},
	{
	    ID:       89,
	    Question: "Which action can reduce the risk of ESD damage when computer equipment is being worked on?",
	    Options: []string{
	        "Working on a grounded antistatic mat",
	        "moving cordless phones away from the work area",
	        "keeping the computer plugged into a surge protector",
	        "lowering the humidity level in the work area",
	    },
	    Answer: 0, // grounded antistatic mat
	},
	{
	    ID:       90,
	    Question: "Which adapter would a technician install in a desktop computer to enable a video signal to be recorded from a video recorder to the computer hard drive?",
	    Options: []string{
	        "video capture card",
	        "video adapter",
	        "TV tuner card",
	        "network interface card",
	    },
	    Answer: 0, // video capture card
	},
	{
	    ID:       91,
	    Question: "Which characteristic describes a DIP?",
	    Options: []string{
	        "an individual memory chip that has dual rows of pins used to attach it to the motherboard",
	        "chips that require constant power to function and are often used for cache memory",
	        "chips that run at clock speeds of 800 MHz and have a connector with 240 pins",
	        "chips whose contents can be “flashed” for deletion and are often used to store BIOS",
	    },
	    Answer: 0, // dual in-line package
	},
	{
	    ID:       92,
	    Question: "Which characteristic describes DDR3 SDRAM?",
	    Options: []string{
	        "chips that run at clock speeds of 800 MHz and have a connector with 240 pins",
	        "an individual memory chip that has dual rows of pins used to attach it to the motherboard",
	        "a small circuit board that holds several memory chips and has a 30- or 72-pin configuration",
	        "chips specifically designed for video graphics that are used in conjunction with a dedicated GPU",
	    },
	    Answer: 0, // DDR3 characteristics
	},
	{
	    ID:       93,
	    Question: "Which characteristic describes ECC memory?",
	    Options: []string{
	        "chips specifically designed for video graphics that are used in conjunction with a dedicated GPU",
	        "chips that can detect multiple bit errors and correct single bit errors in memory",
	        "an individual memory chip that has dual rows of pins used to attach it to the motherboard",
	        "chips that run at clock speeds of 800 MHz and have a connector with 240 pins",
	    },
	    Answer: 1, // ECC functionality
	},
	{
	    ID:       94,
	    Question: "Which characteristic describes EPROM?",
	    Options: []string{
	        "SRAM that is internal and integrated into the CPU",
	        "a smaller, more condensed memory module that provides random access data storage, ideal for use in laptops, printers, and other devices where conserving space is desirable",
	        "a chip that is nonvolatile and can be erased by exposing it to strong ultraviolet light",
	        "a small circuit board that holds several memory chips and has a 30- or 72-pin configuration",
	    },
	    Answer: 2, // UV-erasable ROM
	},
	{
	    ID:       95,
	    Question: "Which characteristic describes EPROM?",
	    Options: []string{
	        "chips that run at clock speeds of 800 MHz and have a connector with 240 pins",
	        "chips specifically designed for video graphics that are used in conjunction with a dedicated GPU",
	        "chips that require constant power to function and are often used for cache memory",
	        "a chip that is nonvolatile and can be erased by exposing it to strong ultraviolet light",
	    },
	    Answer: 3, // UV-erasable ROM
	},
	{
	    ID:       96,
	    Question: "Which characteristic describes GDDR SDRAM?",
	    Options: []string{
	        "chips that require constant power to function and are often used for cache memory",
	        "chips that run at clock speeds of 800 MHz and have a connector with 240 pins",
	        "chips whose contents can be “flashed” for deletion and are often used to store BIOS",
	        "chips specifically designed for video graphics that are used in conjunction with a dedicated GPU",
	    },
	    Answer: 3, // graphics DDR
	},
	{
	    ID:       97,
	    Question: "Which characteristic describes PROM?",
	    Options: []string{
	        "chips that run at clock speeds of 800 MHz and have a connector with 240 pins",
	        "chips specifically designed for video graphics that are used in conjunction with a dedicated GPU",
	        "chips that require constant power to function and are often used for cache memory",
	        "chips that are manufactured blank and then can be programmed once by a PROM programmer",
	    },
	    Answer: 3, // programmable once
	},
	{
	    ID:       98,
	    Question: "Which characteristic describes ROM?",
	    Options: []string{
	        "chips that run at clock speeds of 800 MHz and have a connector with 240 pins",
	        "chips specifically designed for video graphics that are used in conjunction with a dedicated GPU",
	        "chips that require constant power to function and are often used for cache memory",
	        "a chip that cannot be erased or rewritten and is now obsolete",
	    },
	    Answer: 3, // traditional ROM
	},
	{
	    ID:       99,
	    Question: "Which condition refers to a reduced voltage level of AC power that lasts for an extended period of time?",
	    Options: []string{
	        "spike",
	        "sag",
	        "surge",
	        "brownout",
	    },
	    Answer: 3, // brownout
	},
	{
	    ID:       100,
	    Question: "Which device provides wireless connectivity to users as its primary function?",
	    Options: []string{
	        "router",
	        "modem",
	        "access point",
	        "switch",
	    },
	    Answer: 2, // access point
	},
	{
	    ID:       101,
	    Question: "Which filtering method uses the physical address to specify exactly which device is allowed or blocked from sending data on a network?",
	    Options: []string{
	        "port triggering",
	        "MAC address filtering",
	        "port forwarding",
	        "whitelisting",
	    },
	    Answer: 1, // MAC filtering
	},
	{
	    ID:       102,
	    Question: "Which IEEE standard operates at wireless frequencies in both the 5 GHz and 2.4 GHz ranges?",
	    Options: []string{
	        "802.11a",
	        "802.11n",
	        "802.11b",
	        "802.11g",
	    },
	    Answer: 1, // 802.11n
	},
	{
	    ID:       103,
	    Question: "Which is measured in GHz?",
	    Options: []string{
	        "Processor",
	        "Hard Disk",
	        "RAM",
	        "Power Supply",
	    },
	    Answer: 0, // Processor
	},
	{
	    ID:       104,
	    Question: "Which is not the best example of a peripheral?",
	    Options: []string{
	        "Printer",
	        "Speakers",
	        "Hard Disk",
	        "Keyboard",
	    },
	    Answer: 2, // Hard Disk
	},
	{
	    ID:       105,
	    Question: "Which is the main circuit board of the computer?",
	    Options: []string{
	        "Motherboard",
	        "RAM",
	        "ROM",
	        "CPU",
	    },
	    Answer: 0, // Motherboard
	},
	{
	    ID:       106,
	    Question: "Which is true about power supply?",
	    Options: []string{
	        "A single PSU can supply 5 computer units",
	        "It converts the main alternating current into low-voltage direct current.",
	        "It has its own processor",
	        "PSU's are all true rated",
	    },
	    Answer: 1, // AC to DC conversion
	},
	{
	    ID:       107,
	    Question: "Which motherboard form factor has the smallest footprint for use in thin client devices?",
	    Options: []string{
	        "ATX",
	        "Mini-ATX",
	        "ITX",
	        "Micro-ATX",
	    },
	    Answer: 1, // Mini-ATX (per given key)
	},
	{
	    ID:       108,
	    Question: "Which network protocol is used to automatically assign an IP address to a computer on a network?",
	    Options: []string{
	        "APIPA",
	        "DHCP",
	        "ICMP",
	        "SMTP",
	        "FTP",
	    },
	    Answer: 1, // DHCP
	},
	{
	    ID:       109,
	    Question: "Which network server is malfunctioning if a user can ping the IP address of a web server but cannot ping the web server host name?",
	    Options: []string{
	        "the DHCP server",
	        "the HTTP server",
	        "the DNS server",
	        "the FTP server",
	    },
	    Answer: 2, // DNS server
	},
	{
	    ID:       110,
	    Question: "Which of the following is an example of a network layer (layer 3) protocol?",
	    Options: []string{
	        "Ethernet",
	        "TCP",
	        "UDP",
	        "IP",
	    },
	    Answer: 3, // IP
	},
	{
	    ID:       111,
	    Question: "Which one of the below is classed as volatile storage?",
	    Options: []string{
	        "RAM",
	        "SSD",
	        "HDD",
	        "Memory Stick",
	    },
	    Answer: 0, // RAM
	},
	{
	    ID:       112,
	    Question: "Which one of these devices is optical?",
	    Options: []string{
	        "CD",
	        "SSD",
	        "USB flash drive",
	        "Magnetic stripe",
	    },
	    Answer: 0, // CD
	},
	{
	    ID:       113,
	    Question: "Which pairs of wires change termination order between the 568A and 568B standards?",
	    Options: []string{
	        "blue and brown",
	        "green and orange",
	        "green and brown",
	        "brown and orange",
	    },
	    Answer: 1, // green and orange
	},
	{
	    ID:       114,
	    Question: "Which PC component communicates with the CPU through the Southbridge chipset?",
	    Options: []string{
	        "BIOS",
	        "hard drive",
	        "RAM",
	        "video card",
	    },
	    Answer: 1, // hard drive
	},
	{
	    ID:       115,
	    Question: "Which PC motherboard bus is used to connect the CPU to RAM and other motherboard components?",
	    Options: []string{
	        "PCI",
	        "SATA",
	        "front-side",
	        "PCIe",
	    },
	    Answer: 2, // front-side bus
	},
	{
	    ID:       116,
	    Question: "Which port allows for the transmission of high definition video using the DisplayPort protocol?",
	    Options: []string{
	        "DVI",
	        "VGA",
	        "RCA",
	        "Thunderbolt",
	    },
	    Answer: 3, // Thunderbolt
	},
	{
	    ID:       117,
	    Question: "Which smart home wireless technology has an open standard that allows up to 232 devices to be connected?",
	    Options: []string{
	        "Z-Wave",
	        "Zigbee",
	        "802.11n",
	        "802.11ac",
	    },
	    Answer: 0, // Z-Wave
	},
	{
	    ID:       118,
	    Question: "Which statement describes a characteristic of SRAM in a PC?",
	    Options: []string{
	        "It has the highest power consumption.",
	        "It is used for cache memory.",
	        "It has a connector with 240 pins.",
	        "It is used as main RAM in a PC.",
	    },
	    Answer: 1, // cache memory
	},
	{
	    ID:       119,
	    Question: "Which statement describes Augmented Reality (AR) technology?",
	    Options: []string{
	        "It superimposes images and audio over the real world in real time.",
	        "The headset closes off any ambient light to users.",
	        "It always requires a headset.",
	        "It does not provide users with immediate access to information about their real surroundings.",
	    },
	    Answer: 0, // AR definition
	},
	{
	    ID:       120,
	    Question: "Which statement describes the purpose of an I/O connector plate?",
	    Options: []string{
	        "It makes the I/O ports of the motherboard available for connection in a variety of computer cases.",
	        "It provides multiple connections for SATA hard drives to connect to the motherboard.",
	        "It plugs into the motherboard and expands the number of available slots for adapter cards.",
	        "It connects the PCIe adapter slots used for video directly to the CPU for faster processing.",
	    },
	    Answer: 0, // I/O shield purpose
	},
	{
	    ID:       121,
	    Question: "Which tool can protect computer components from the effects of ESD?",
	    Options: []string{
	        "antistatic wrist strap",
	        "surge suppressor",
	        "UPS",
	        "SPS",
	    },
	    Answer: 0, // antistatic wrist strap
	},
	{
	    ID:       122,
	    Question: "Which tool can protect computer components from the effects of ESD?",
	    Options: []string{
	        "antistatic wrist strap",
	        "surge suppressor",
	        "SPS",
	        "UPS",
	    },
	    Answer: 0, // antistatic wrist strap
	},
	{
	    ID:       123,
	    Question: "Which type of device would be used on a laptop to verify the identity of a user?",
	    Options: []string{
	        "a MIDI device",
	        "a biometric identification device",
	        "a digitizer",
	        "a touch screen",
	    },
	    Answer: 1, // biometric device
	},
	{
	    ID:       124,
	    Question: "Which type of drive is typically installed in a 5.25 inch (13.34 cm) bay?",
	    Options: []string{
	        "hard drive",
	        "SSD",
	        "optical drive",
	        "flash drive",
	    },
	    Answer: 2, // optical drive
	},
	{
	    ID:       125,
	    Question: "Which type of interface was originally developed for high-definition televisions and is also popular to use with computers to connect audio and video devices?",
	    Options: []string{
	        "FireWire",
	        "USB",
	        "HDMI",
	        "VGA",
	    },
	    Answer: 2, // HDMI
	},
	{
	    ID:       126,
	    Question: "Which type of interface was originally developed for high-definition televisions and is also popular to use with computers to connect audio and video devices?",
	    Options: []string{
	        "HDMI",
	        "FireWire",
	        "DVI",
	        "VGA",
	        "USB",
	    },
	    Answer: 0, // HDMI
	},
	{
	    ID:       127,
	    Question: "Which type of motherboard expansion slot has four types ranging from x1 to x16 with each type having a different length of expansion slot?",
	    Options: []string{
	        "AGP",
	        "PCI",
	        "SATA",
	        "PCIe",
	    },
	    Answer: 3, // PCIe
	},
	{
	    ID:       128,
	    Question: "Which type of motherboard expansion slot sends data one bit at a time over a serial bus?",
	    Options: []string{
	        "PCIe",
	        "PCI",
	        "RAM",
	        "PATA",
	    },
	    Answer: 0, // PCIe
	},
	{
	    ID:       129,
	    Question: "Why is it important to ground both computers and network devices?",
	    Options: []string{
	        "to facilitate the flow of current from the power supply to the computer case",
	        "to provide a path of least resistance for stray current",
	        "to ensure that both the power supplied and the power used is in sync with the ground voltage",
	        "to ensure that the power supply is limited to an output of 110V DC",
	    },
	    Answer: 1, // path of least resistance
	},
	}

var store = NewTestStore(30 * time.Minute)

func main() {
	rand.Seed(time.Now().UnixNano())

	mux := http.NewServeMux()
	mux.HandleFunc("/start", startHandler)
	mux.HandleFunc("/submit", submitHandler)

	// CORS для локального фронта
	handler := withCORS(mux)

	// Периодическая очистка протухших тестов
	go func() {
		t := time.NewTicker(5 * time.Minute)
		for range t.C {
			store.CleanupExpired()
		}
	}()

	log.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Разрешаем фронту с другого origin
		w.Header().Set("Access-Control-Allow-Origin", "https://uraniumcore.github.io")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"success": false,
			"error":   "Method Not Allowed",
		})
		return
	}

	var req StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "invalid json",
		})
		return
	}

	// Здесь можно загрузить из файла вместо baseQuestions.
	// Пример для файла:
	//   f, err := os.Open("test.json")
	//   ... json.NewDecoder(f).Decode(&baseQuestions)
	// [Важно: на фронт не возвращать Answer!]

	// Генерируем test_id (упростим)
	testID := randomTestID()

	// Сохраняем полный список (с Answer) в store
	store.Put(testID, baseQuestions)

	// Формируем публичные вопросы для фронта
	pub := make([]PublicQuestion, len(baseQuestions))
	for i, q := range baseQuestions {
		pub[i] = PublicQuestion{
			ID:       q.ID,
			Question: q.Question,
			Options:  q.Options,
		}
	}

	resp := StartResponse{
		Success:   true,
		TestID:    testID,
		Questions: pub,
	}
	writeJSON(w, http.StatusOK, resp)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"success": false,
			"error":   "Method Not Allowed",
		})
		return
	}

	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "invalid json",
		})
		return
	}

	// Достаем серверные правильные ответы по test_id
	qs, ok := store.Get(req.TestID)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"success": false,
			"error":   "invalid or expired test_id",
		})
		return
	}

	// Индексируем по id
	qByID := make(map[int]Question, len(qs))
	for _, q := range qs {
		qByID[q.ID] = q
	}

	score := 0
	review := make([]ReviewItem, 0, len(req.Answers))

	for _, a := range req.Answers {
		q, exists := qByID[a.QuestionID]
		if !exists {
			// неизвестный id — пропускаем
			continue
		}
		if a.Choice == q.Answer {
			score++
		}
		review = append(review, ReviewItem{
			QuestionID:    q.ID,
			Question:      q.Question,
			Options:       q.Options,
			CorrectChoice: q.Answer,
			UserChoice:    a.Choice,
		})
	}

	resp := SubmitResponse{
		Success: true,
		Score:   score,
		Total:   len(qs),
		Results: review,
	}
	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func randomTestID() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 10)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "test-" + string(b)
}
