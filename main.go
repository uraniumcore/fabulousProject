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
		w.Header().Set("Access-Control-Allow-Origin", "https://uraniumcore.github.io/fabulousProject/")
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
