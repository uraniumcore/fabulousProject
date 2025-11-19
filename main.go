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
		Question: "Picture 4.2. Which port on the switch is used to connect computer/laptop with Ethernet cable?",
		Options: []string{
			"1",
			"2",
			"3",
		},
		Answer: -1, // depends on Picture 4.2, not determinable without image
	},
	{
		ID:       2,
		Question: "Inside Laser Unit. Which element is responsible for reflecting laser string out of the Laser Unit to Imaging Drum?",
		Options: []string{
			"Spinning Mirror",
			"Mirror",
			"Lense #1",
			"Lense #2",
		},
		Answer: 1, // Mirror
	},
	{
		ID:       3,
		Question: "Picture 1.1. Which of the following is the port of 'Receiving signals' for RJ45 on the main module of the cable tester?",
		Options: []string{
			"1",
			"2",
			"3",
			"4",
		},
		Answer: -1, // depends on Picture 1.1, not determinable without image
	},
	{
		ID:       4,
		Question: "Which hypervisor should be used while organizing hosted virtualization?",
		Options: []string{
			"VMware ESXi",
			"VMware Workstation",
			"Windows 10",
			"Windows XP",
		},
		Answer: 1, // VMware Workstation is a hosted (Type 2) hypervisor
	},
	{
		ID:       5,
		Question: "Picture 5.3. Which of the following is the analog camera? (Two dome cameras shown, one with RJ-45, one with BNC connector.)",
		Options: []string{
			"Right",
			"Left",
		},
		Answer: -1, // needs Picture 5.3 to know which side has the BNC (analog) connector
	},
	{
		ID:       6,
		Question: "Which tool can be used to perform crimping?",
		Options: []string{
			"Cable Tester",
			"Crimping Tool",
			"Pinout",
			"Pliers (Plioskogubtsy)",
		},
		Answer: 1, // Crimping Tool
	},
	{
		ID:       7,
		Question: "Which hypervisor should be used while organizing native virtualization?",
		Options: []string{
			"VMware ESXi",
			"VMware Workstation",
			"Windows 10",
			"Windows XP",
		},
		Answer: 0, // VMware ESXi is a native (Type 1) hypervisor
	},
	{
		ID:       8,
		Question: "Which term defines the right sequence of the twisted pair wires inside the connector?",
		Options: []string{
			"Colorcode",
			"Pinout",
			"Pin",
			"Code",
		},
		Answer: 1, // Pinout
	},
	{
		ID:       9,
		Question: "Situation: You are using 32-bit Operating System. You need to install application or equipment which only supports 64-bit Operating System. What is one way to completely resolve the issue and be able to use those applications or equipment?",
		Options: []string{
			"Reinstall OS",
			"Use Virtual Machine",
			"Upgrade Operating System",
			"Downgrade Operating System",
		},
		Answer: 2, // Upgrade Operating System (to a 64-bit edition)
	},
	{
		ID:       10,
		Question: "How can you check for connection with a switch management thru the serial connection?",
		Options: []string{
			"PING",
			"ipconfig",
			"impossible",
			"ipconfig /all",
		},
		Answer: 2, // impossible (serial is not an IP connection you can ping)
	},
	{
		ID:       11,
		Question: "How can you check IP address assignment on your laptop (brief)?",
		Options: []string{
			"PING",
			"ipconfig",
			"impossible",
			"ipconfig /all",
		},
		Answer: 1, // ipconfig
	},
	{
		ID:       12,
		Question: "Which power connector is retired connector for HDDs and Optical Drives?",
		Options: []string{
			"Molex",
			"Berg",
			"SATA Power",
			"4Pin",
		},
		Answer: 0, // Molex
	},
	{
		ID:       13,
		Question: "What is the measurement for the speed of rotating motor in the HDD?",
		Options: []string{
			"RPM",
			"RPS",
			"KM/H",
			"M/S",
		},
		Answer: 0, // RPM
	},
	{
		ID:       14,
		Question: "Picture 3.1. Which of the following elements are 'Spinning Mirror'? (Elements numbered 1–10 in the laser unit.)",
		Options: []string{
			"1 and 6",
			"2 and 7",
			"3 and 8",
			"4 and 9",
		},
		Answer: -1, // depends on Picture 3.1, not determinable without image
	},
	{
		ID:       15,
		Question: "Which parameters you should know to establish Telnet connection with a switch?",
		Options: []string{
			"IP address, port number from device manager",
			"Baud Rate, COM number from device manager",
			"IP address, default port number by default",
			"Speed, COM number by default",
		},
		Answer: 2, // IP address and default Telnet port (23)
	},
	{
		ID:       16,
		Question: "You configuring DHCP server. How should you assign IP for yourself?",
		Options: []string{
			"static IP",
			"dynamic IP",
		},
		Answer: 0, // static IP
	},
	{
		ID:       17,
		Question: "What is the mixture of UTP + RJ45 on both ends?",
		Options: []string{
			"Ethernet cable",
			"Connector",
			"Wire",
			"Tool",
		},
		Answer: 0, // Ethernet cable
	},
	{
		ID:       18,
		Question: "Situation: You need to print out a certificate. You use a specific paper for that, which is very expensive. Your paper tray is full of regular paper. What should you do?",
		Options: []string{
			"Unload paper tray, and load specific paper",
			"Put specific paper on top of paper tray",
			"Use manual feed tray",
			"Use paper tray, instead of manual feed tray",
		},
		Answer: 2, // Use manual feed tray
	},
	{
		ID:       19,
		Question: "Picture 5.4. What is listed in the following window? (IP camera management software with a list of URLs.)",
		Options: []string{
			"IP cameras list",
			"User accounts",
			"FTP sessions",
			"Virtual machines",
		},
		Answer: 0, // IP cameras list
	},
	{
		ID:       20,
		Question: "On the HDD picture, what is the measurement unit typically used for spindle motor speed specification?",
		Options: []string{
			"RPM",
			"RPS",
			"KM/H",
			"M/S",
		},
		Answer: 0, // RPM
	},
	{
		ID:       21,
		Question: "Picture 3.1. Which of the following elements are 'Spinning Mirror'? (Elements numbered 1–10 in the laser unit.)",
		Options: []string{
			"1 and 6",
			"2 and 7",
			"3 and 8",
			"4 and 9",
		},
		Answer: -1, // depends on Picture 3.1, not determinable without the specific diagram
	},
	{
		ID:       22,
		Question: "Picture 6.1. What is the IP address of management of ESXi hypervisor? (On screen: 'Download tools to manage this host from: http://192.168.205.120/ (VMCP)' and vSphere Client IP field.)",
		Options: []string{
			"DHCP",
			"Not assigned",
			"192.168.205.120",
			"192.160.105",
		},
		Answer: 2, // 192.168.205.120
	},
	{
		ID:       23,
		Question: "Picture 1.1. Which of the following is the port of 'Receiving signals' for RJ45 on the main module of the cable tester? (Ports numbered 1–7 around the tester.)",
		Options: []string{
			"1",
			"2",
			"3",
			"4",
			"5",
			"6",
			"7",
		},
		Answer: -1, // depends on Picture 1.1, not determinable without the specific tester layout
	},
	{
		ID:       24,
		Question: "Which hypervisor should be used while organizing bare metal virtualization?",
		Options: []string{
			"VMware ESXi",
			"VMware Workstation",
			"Windows 10",
			"Windows XP",
		},
		Answer: 0, // VMware ESXi (Type 1 / bare metal hypervisor)
	},
	{
		ID:       25,
		Question: "What is the common name of virtualization software?",
		Options: []string{
			"Hypervisor",
			"Hyper-V",
			"VMware Workstation",
			"Hyperterminal",
		},
		Answer: 0, // Hypervisor
	},
	{
		ID:       26,
		Question: "Picture 7.1. You are about to test virtual environment. You need fastest way to run any OS on VM. Which option would you choose? (VMware Workstation – selecting ISO image for installation.)",
		Options: []string{
			"Boot from physical DVD drive",
			"Use ISO image file",
			"Boot from network (PXE)",
			"Use existing virtual disk",
		},
		Answer: 1, // Use ISO image file
	},
	{
		ID:       27,
		Question: "Inside Laser Unit. Which element is responsible for magnifying the laser string?",
		Options: []string{
			"Spinning Mirror",
			"Mirror",
			"Lense #1",
			"Lense #2",
		},
		Answer: 2, // Lense #1 (lens does magnification)
	},
	{
		ID:       28,
		Question: "Inside Laser Unit. Which element is responsible for spreading laser beam into a string?",
		Options: []string{
			"Spinning Mirror",
			"Mirror",
			"Lense #1",
			"Lense #2",
		},
		Answer: 0, // Spinning Mirror (polygon mirror scans beam into a line/string)
	},
	{
		ID:       29,
		Question: "Which is Crimper?",
		Options: []string{
			"Pliers (Plioskogubtsy)",
			"Connector",
			"Wire",
			"Tool",
		},
		Answer: 3, // Tool
	},
	{
		ID:       30,
		Question: "Picture 4.1. Which Network Adapter is virtual and could be considered as the consequence of using virtualization? (Network Connections window, adapters numbered 1–6, several with 'VMware Network Adapter' in the name.)",
		Options: []string{
			"1",
			"2",
			"3",
			"4",
			"5",
			"6",
		},
		Answer: -1, // depends on which numbered item in Picture 4.1 is labeled 'VMware Network Adapter'
	},
	{
		ID:       31,
		Question: "Picture 5.3. Match the items with purpose: two dome cameras – left with RJ-45 and DC12V, right with BNC Connector and DC12V.",
		Options: []string{
			"Left – IP camera; Right – analog camera",
			"Left – analog camera; Right – IP camera",
			"Both – IP cameras",
			"Both – analog cameras",
		},
		Answer: 0, // Left – IP camera (RJ-45); Right – analog camera (BNC)
	},
	{
		ID:       32,
		Question: "You configured DHCP server. How can you identify the host and understand which IP is assigned for it?",
		Options: []string{
			"by dynamic IP address",
			"by MAC address",
			"by PC model",
			"by static IP address",
		},
		Answer: 1, // by MAC address
	},
	{
		ID:       33,
		Question: "Picture 8.6. Which option would you choose to boot to existing OS? (Boot Menu: 1. Removable Devices, 2. Hard Drive, 3. CD-ROM Drive, 4. Network boot.)",
		Options: []string{
			"1",
			"2",
			"3",
			"4",
		},
		Answer: 1, // Hard Drive
	},
	{
		ID:       34,
		Question: "Which output from cmd indicates successful answer from ping request?",
		Options: []string{
			"Reply from 192.168.1.1: bytes=32 time<1 ms TTL=255",
			"Request timed out",
			"Reply from 64.100.0.1: Destination host unreachable",
			"Request successful",
		},
		Answer: 0, // successful ping has 'Reply from ... bytes=32 time=... TTL=...'
	},
	{
		ID:       35,
		Question: "Picture 4.1. Which Network Adapter is virtual and could be considered as the consequences of using virtualization? (Network Connections window with adapters numbered 1–6, VMware adapters among them.)",
		Options: []string{
			"1",
			"2",
			"3",
			"4",
			"5",
			"6",
		},
		Answer: -1, // depends on which numbers correspond to 'VMware Network Adapter' in the picture
	},
	{
		ID:       36,
		Question: "Situation: You need to access switch management. You just purchased new switch from store. What is the easiest way to find out the IP address?",
		Options: []string{
			"Read manual and find default IP address",
			"Use console port to verify the IP address",
			"Read manual to find the correct Baud Rate",
			"Use network connection to access switch",
		},
		Answer: 0, // read manual for default management IP
	},
	{
		ID:       37,
		Question: "Picture 2.2. Options. Which board does not contain any malfunctions and can likely be used? (Photo of several PCBs, one without bulging/leaking capacitors.)",
		Options: []string{
			"Board A",
			"Board B",
			"Board C",
			"Board D",
		},
		Answer: -1, // depends on the actual photo of the boards
	},
	{
		ID:       38,
		Question: "Picture 8.6. Which option would you choose to boot from ISO image? (Boot Menu: 1. Removable Devices, 2. Hard Drive, 3. CD-ROM Drive, 4. Network boot from AMD adapter.)",
		Options: []string{
			"1",
			"2",
			"3",
			"4",
		},
		Answer: 3, // CD-ROM Drive (typical for mounted ISO on many VMs)
	},
	{
		ID:       39,
		Question: "Picture 6.2. What is the password you are setting during FileZilla Server installation?",
		Options: []string{
			"password of FTP server",
			"password of FTP server administration",
			"password of Web server",
			"password of Web server administration",
		},
		Answer: 1, // FTP server administration password
	},
	{
		ID:       40,
		Question: "Picture 1.1. Which of the following is the port of 'Transferring signals' for RJ11? (Cable tester views with ports numbered 1–7.)",
		Options: []string{
			"1",
			"2",
			"3",
			"4",
			"5",
			"6",
			"7",
		},
		Answer: -1, // depends on the specific tester layout in the picture
	},
	{
		ID:       41,
		Question: "Which hypervisor should be used while organizing hosted virtualization?",
		Options: []string{
			"VMware ESXi",
			"VMware Workstation",
			"Windows 10",
			"Windows XP",
		},
		Answer: 1, // VMware Workstation (hosted / Type 2 hypervisor)
	},
	{
		ID:       42,
		Question: "What are the main functions of IIS?",
		Options: []string{
			"FTP server, Web server",
			"FTP server, FTP client",
			"Web server, Web client",
			"FTP client, Web client",
		},
		Answer: 0, // IIS provides Web and FTP server roles
	},
	{
		ID:       43,
		Question: "Which power connector is retired connector for HDDs and Optical Drives?",
		Options: []string{
			"Molex",
			"Berg",
			"SATA Power",
			"4Pin",
		},
		Answer: 0, // Molex (legacy 4-pin peripheral power)
	},
	{
		ID:       44,
		Question: "Picture 5.2. Situation: You are sending ping request to switch in LAN. Your IP address: 192.168.255.15, switch IP address: 192.168.225.45. All your network cards are active. The CMD window shows 'Destination host unreachable' from another IP. What is the problem?",
		Options: []string{
			"Ping answer is successful",
			"Ping sending packets thru wrong network card",
			"Ping is sent to wrong IP",
			"Your IP is in wrong subnet",
		},
		Answer: 1, // reply from another local IP with 'Destination host unreachable' ⇒ wrong NIC / route
	},
	{
		ID:       45,
		Question: "What is RJ45?",
		Options: []string{
			"Ethernet cable",
			"Connector",
			"Wire",
			"Tool",
		},
		Answer: 1, // RJ45 is the connector
	},
	{
		ID:       46,
		Question: "Which PCB responsible for logical processing in the printer?",
		Options: []string{
			"Formatter (Green Board)",
			"PS Board (Yellow Board)",
			"Power Supply",
			"Mother Board",
		},
		Answer: 0, // Formatter board handles printer logic / processing
	},
	{
		ID:       47,
		Question: "Picture 1.1. Which of the following is the port of 'Receiving signals' for RJ45 on the main module? (Cable tester with numbered ports 1–7.)",
		Options: []string{
			"1",
			"2",
			"3",
			"4",
			"5",
			"6",
			"7",
		},
		Answer: -1, // depends on the specific tester diagram
	},
	{
		ID:       48,
		Question: "Picture 5.3. Match the items with its purpose. Two cameras: left with RG-45 and DC12V, right with BNC Connector and DC12V.",
		Options: []string{
			"DC12v – to power supply, RJ45 – to LAN, BNC – to analog recorder",
			"BNC – to power supply, RJ45 – to LAN, DC12v – to analog recorder",
			"DC12v – to power supply, BNC – to LAN, RJ45 – to analog recorder",
			"RJ45 – to power supply, DC12v – to LAN, BNC – to analog recorder",
		},
		Answer: 0, // DC12V is power, RJ45 is LAN (IP cam), BNC is coax to analog recorder
	},
	{
		ID:       49,
		Question: "What are the main parameters when configuring FTP Server?",
		Options: []string{
			"Login credentials, assigned directory",
			"Login, password",
			"Assigned directory",
			"Root directory",
		},
		Answer: 0, // need credentials + directory to share
	},
	{
		ID:       50,
		Question: "Which hypervisor should be used while organizing hosted virtualization?",
		Options: []string{
			"VMware ESXi",
			"VMware Workstation",
			"Windows 10",
			"Windows XP",
		},
		Answer: 1, // VMware Workstation (hosted / Type 2)
	},
	{
		ID:       51,
		Question: "Picture 8.5. Which key will you use to enter BIOS? (Text on screen: 'Press F2 to enter SETUP, F12 for Network Boot, ESC for Boot Menu'.)",
		Options: []string{
			"F2",
			"F12",
			"ESC",
			"Tab",
		},
		Answer: 0, // F2
	},
	{
		ID:       52,
		Question: "Which hypervisor should be used while organizing native virtualization?",
		Options: []string{
			"VMware ESXi",
			"VMware Workstation",
			"Windows 10",
			"Windows XP",
		},
		Answer: 0, // VMware ESXi (native / Type 1)
	},
	{
		ID:       53,
		Question: "Which term defines the right sequence of the twisted pair wires inside the connector?",
		Options: []string{
			"Colorcode",
			"Pinout",
			"Pin",
			"Code",
		},
		Answer: 1, // Pinout
	},
	{
		ID:       54,
		Question: "Situation: You are using 32-bit Operating System. You need to install application or equipment which only supports 64-bit Operating System. What is one way to completely resolve the issue and be able to use those applications?",
		Options: []string{
			"Reinstall OS",
			"Use Virtual Machine",
			"Upgrade Operating System",
			"Downgrade Operating System",
		},
		Answer: 2, // Upgrade Operating System (to 64-bit)
	},
	{
		ID:       55,
		Question: "How can you check for connection with a switch management thru the serial connection?",
		Options: []string{
			"PING",
			"ipconfig",
			"impossible",
			"ipconfig /all",
		},
		Answer: 2, // impossible (serial is not IP-based)
	},
	{
		ID:       56,
		Question: "What is the measurement for the speed of rotating motor in the HDD?",
		Options: []string{
			"RPM",
			"RPS",
			"KM/H",
			"M/S",
		},
		Answer: 0, // RPM
	},
	{
		ID:       57,
		Question: "Picture 3.1. Which of the following elements are 'Spinning Mirror'? (Elements numbered 1–10 in the laser unit.)",
		Options: []string{
			"1 and 6",
			"2 and 7",
			"3 and 8",
			"4 and 9",
		},
		Answer: -1, // depends on Picture 3.1, not visible here
	},
	{
		ID:       58,
		Question: "In FileZilla server user configuration window, which user has access to folder D:\\test? (User list shows 'system user', 'system user2', 'test', 'test2'.)",
		Options: []string{
			"system user",
			"system user2",
			"test",
			"test2",
		},
		Answer: 2, // test (by typical naming in such task)
	},
	{
		ID:       59,
		Question: "How can you check IP address assignment on your laptop (brief)?",
		Options: []string{
			"PING",
			"ipconfig",
			"impossible",
			"ipconfig /all",
		},
		Answer: 1, // ipconfig
	},
	{
		ID:       60,
		Question: "Which power connector is retired connector for HDDs and Optical Drives?",
		Options: []string{
			"Molex",
			"Berg",
			"SATA Power",
			"4Pin",
		},
		Answer: 0, // Molex
	},
	{
		ID:       61,
		Question: "Picture 5.4. What is listed in the following window? (IP camera software showing 'Connect to IP Cameras' with URLs list.)",
		Options: []string{
			"List of IP cameras",
			"List of FTP servers",
			"List of virtual machines",
			"List of users",
		},
		Answer: 0, // List of IP cameras
	},
	{
		ID:       62,
		Question: "Situation: You need to print out a certificate. You use a specific paper for that, which is very expensive. Your paper tray is full of regular paper. What should you do?",
		Options: []string{
			"Unload paper tray, and load specific paper",
			"Put specific paper on top of paper tray",
			"Use manual feed tray",
			"Use paper tray, instead of manual feed tray",
		},
		Answer: 2, // Use manual feed tray
	},
	{
		ID:       63,
		Question: "Picture 3.1. Which of the following elements are 'Lense #2'?",
		Options: []string{
			"1 and 6",
			"2 and 7",
			"3 and 8",
			"4 and 9",
		},
		Answer: -1, // depends on Picture 3.1, not visible here
	},
	{
		ID:       64,
		Question: "Picture 4.2. Which port on the switch is used to connect computer/laptop with Ethernet cable? (Ports group numbered 1–3.)",
		Options: []string{
			"1",
			"2",
			"3",
		},
		Answer: -1, // depends on Picture 4.2 layout
	},
	{
		ID:       65,
		Question: "Picture 1.1. Which of the following is the port of 'Receiving signals' for RJ45 on the main module?",
		Options: []string{
			"1",
			"2",
			"3",
			"4",
		},
		Answer: -1, // depends on that specific tester diagram
	},
	{
		ID:       66,
		Question: "What are the main parameters when configuring FTP Server?",
		Options: []string{
			"Login credentials, assigned directory",
			"Login, password",
			"Assigned directory",
			"Root directory",
		},
		Answer: 0, // Login credentials, assigned directory
	},
	{
		ID:       67,
		Question: "Situation: You need to print out a certificate. You use a specific paper for that, which is very expensive. Your paper tray is full of regular paper. What should you do?",
		Options: []string{
			"Unload paper tray, and load specific paper",
			"Put specific paper on top of paper tray",
			"Use manual feed tray",
			"Use paper tray, instead of manual feed tray",
		},
		Answer: 2, // Use manual feed tray
	},
	{
		ID:       68,
		Question: "How can you create LiveUSB?",
		Options: []string{
			"Write bootable image to USB Stick",
			"Unarchive image and copy files to USB Stick",
			"Copy image to USB Stick",
		},
		Answer: 0, // Write bootable image to USB Stick
	},
	{
		ID:       69,
		Question: "What are the main functions of IIS?",
		Options: []string{
			"FTP server, Web server",
			"FTP server, FTP client",
			"Web server, Web client",
			"FTP client, Web client",
		},
		Answer: 0, // FTP server, Web server
	},
	{
		ID:       70,
		Question: "You configured DHCP server. How can you identify the host and understand which IP is assigned for it?",
		Options: []string{
			"by dynamic IP address",
			"by MAC address",
			"by PC model",
			"by static IP address",
		},
		Answer: 1, // by MAC address
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
