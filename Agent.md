

# **Technical Project: "Roster Bot" Duty Schedule Management System**

## **I. Strategic Overview: System Architecture and Technology Stack**

This section presents the high-level architectural design of the system, provides a rationale for the selection of key technologies, and defines the structural organization of the project. This section lays the strategic foundation for all subsequent implementation phases.

### **1.1. System Architectural Design**

The system is designed as a monolithic application with clear separation of internal modules, deployed as a single Docker container. This architecture ensures simplicity in deployment and maintenance, which is ideal for a project of this scale, while simultaneously allowing for parallel development thanks to strict logical boundaries between components.

The diagram below illustrates the main components of the system and their interactions:

* **User Interaction Layer:** Represented by the Telegram client for the chat interface and a web browser for accessing the mobile-friendly web interface.  
* **Application Layer:** A single Go application comprising three main services running in separate goroutines:  
  1. **Telegram Bot Handler:** Manages all incoming messages and commands from users in Telegram.  
  2. **Web API Server:** Provides a RESTful API for interaction with the web interface. This server will differentiate between public (read-only) access and authenticated access for users interacting via the Telegram Web App.  
  3. **Notification Scheduler (Cron Scheduler):** Responsible for sending daily notifications and automatically assigning duties.  
* **Data Persistence Layer:** Consists of a single SQLite database file, eliminating the need to deploy and administer a separate database server.  
* **Containerization & Orchestration:** Docker is used to package the application and its dependencies into an isolated container, while Docker Compose manages the container's lifecycle and configuration in development and production environments.  
* **CI/CD Pipeline:** GitHub Actions automates the processes of building, testing, and publishing the Docker image to the GitHub Container Registry (GHCR), ensuring continuous integration and delivery.


This model clearly demonstrates the key principles of the system: a single deployment point, separation of interfaces (Telegram and Web), and encapsulation of business logic within the application core.

### **1.2. Technology Stack Selection and Rationale**

The choice of technologies is a fundamental decision that determines the system's performance, reliability, and maintainability. Each technology was selected after a thorough analysis of alternatives, taking into account the specific requirements of the project.

* **Programming Language (Backend): GoLang.** Chosen for its high performance, efficient concurrency model (goroutines), strong static typing, and the ability to compile into a single statically-linked binary. The latter property is critically important for creating minimalist and secure Docker images.  
* **Telegram Bot Library: github.com/go-telegram-bot-api/telegram-bot-api/v5**. This library is a simple and lightweight wrapper around the Telegram Bot API, which does not impose its own architecture.1 Unlike more complex frameworks 3, it provides maximum flexibility and full control over update processing, which perfectly aligns with the principles of clean architecture.  
* **Web API Framework: Gin (github.com/gin-gonic/gin)**. Gin is one of the most performant web frameworks in the Go ecosystem, while also having a minimalist and intuitive API.4 It is ideal for creating the lightweight REST API needed for the web interface and has a large community and extensive documentation.5  
* **SQLite Database Driver: modernc.org/sqlite**. This solution is one of the most significant technical choices in the project. Unlike the popular github.com/mattn/go-sqlite3 driver 6,  
  modernc.org/sqlite is a CGo-free implementation.2 This means that a C compiler (e.g.,  
  gcc) is not required to compile the application. The absence of a CGo dependency radically simplifies the build process in Docker, allowing the use of minimalist base images without installing a full set of build tools (build-essential), which significantly reduces the size and vulnerability of the final Docker image.  
* **Task Scheduler Library: github.com/robfig/cron/v3**. This library is the de-facto standard for working with cron expressions in Go.8 It has a reliable parser, supports standard and extended cron formats, and, critically for this project, supports specifying time zones (IANA Time Zone) directly in the schedule string (e.g.,  
  CRON\_TZ=Europe/Berlin), which ensures the scheduler operates correctly regardless of the server's system time.8  
* **Telegram Web App Authentication: github.com/telegram-mini-apps/init-data-golang**. To securely authenticate users through the Telegram Web App, this library will be used to validate the initialization data (initData) sent from the client.34 It provides a reliable implementation of the validation algorithm specified by Telegram, which involves checking a cryptographic signature against the bot's token.37  
* **Frontend Technologies: Vanilla JS (ES6 Modules) and Tailwind CSS (Standalone CLI)**. This combination fully meets the requirements of the technical specification. Using native ES6 modules allows for structuring the frontend code without introducing heavyweight frameworks like React or Vue.11 Tailwind CSS, in turn, provides a powerful utility-first CSS methodology for creating responsive interfaces. Using it in Standalone CLI mode allows it to be integrated into any project without Node.js dependencies in the production environment.13  
* **DevOps Tools: Docker, Docker Compose, GitHub Actions, Portainer**. This combination represents a modern, proven, and effective stack for automating the application lifecycle. Docker and Docker Compose ensure environment reproducibility, GitHub Actions automates CI/CD 15, and Portainer provides a convenient web interface for managing deployed applications on the server.

The summary table below outlines the technological decisions made.

**Table 1: Technology Stack Decision Matrix**

| Component | Selected Technology | Alternatives Considered | Rationale |
| :---- | :---- | :---- | :---- |
| **Backend Framework** | Go (standard library) | \- | Direct use of the Go standard library for core logic ensures maximum performance and control, avoiding unnecessary abstractions. |
| **Web API Framework** | github.com/gin-gonic/gin | labstack/echo, gorilla/mux 5 | Gin offers an optimal balance of high performance, a minimalist API, and a large community, meeting the need for a simple and fast API server. |
| **Telegram Bot Library** | go-telegram-bot-api/v5 | tucnak/telebot, go-telegram/bot 3 | A thin wrapper over the Telegram API, providing flexibility without imposing architectural constraints, ideal for a clean architecture. |
| **Database & Driver** | SQLite & modernc.org/sqlite | PostgreSQL, mattn/go-sqlite3 6 | SQLite eliminates the need for a separate DB server. The CGo-free driver modernc.org/sqlite 2 is critical for simplifying CI/CD and creating minimal Docker images without a C compiler. |
| **Cron Scheduler** | github.com/robfig/cron/v3 | gdgvda/cron 10 | An industry standard with a reliable implementation, excellent documentation, and crucial support for IANA time zones.8 |
| **Telegram Auth** | telegram-mini-apps/init-data-golang | Custom Implementation, sgzmd/go-telegram-auth 38 | Provides a standard, tested implementation for validating Telegram Web App initData, which is more secure and reliable than a custom solution.34 |
| **Frontend JS** | Vanilla JS (ES6 Modules) | React, Vue, Svelte | Directly meets the requirement. Modern Vanilla JS with modules allows for creating structured and maintainable applications without the overhead of frameworks.11 |
| **Frontend CSS** | Tailwind CSS (Standalone CLI) | Bootstrap, Pure CSS | Tailwind CSS enables rapid development of responsive interfaces. The Standalone CLI 13 allows its use without Node.js dependencies, simplifying the build process. |
| **CI/CD Platform** | GitHub Actions | GitLab CI, Jenkins | Native integration with the GitHub repository, an extensive marketplace of ready-made actions, and robust support for Docker containers.16 |

### **1.3. Project Structure: The Modular Monolith**

To organize the codebase, an approach based on common practices in the Go community, often referred to as the "Standard Go Project Layout," will be applied.17 This structure is not a strict standard but represents a time-tested template for building scalable and maintainable applications. This approach allows for the implementation of the "Modular Monolith" architectural pattern.

The application remains a single binary file (a monolith), which simplifies deployment, but its internal structure is divided into independent modules with clearly defined boundaries. This directly addresses the requirement to "break the project into independent modules for parallel development."

* /cmd: This directory will contain the entry point of the application. For example, /cmd/roster-bot/main.go. The code in this directory will be minimal: its main task is to initialize dependencies (configuration, logger, DB connection), assemble components from the /internal directory, and launch the application.18  
* /internal: This is the core of the application. All business logic, data access implementation, API handlers, and Telegram handlers will be located here. The Go compiler enforces the privacy of this directory: packages inside /internal cannot be imported by other projects. This is a powerful architectural guarantee that prevents the leakage of internal implementation details and allows for free refactoring without the risk of breaking external dependencies.17  
* /web: A dedicated top-level directory for all frontend resources: HTML files, JavaScript source code (/web/js), source CSS files (/web/css), and compiled styles. This separation clearly distinguishes frontend development from backend code.  
* /deployments: This directory will contain all files related to deployment, primarily the Dockerfile and docker-compose.yml. This allows for the separation of infrastructure configuration from the application code.

This approach allows different developers to work on separate packages in /internal (e.g., one on /internal/telegram, another on /internal/http) with minimal risk of conflicts, as interaction between modules occurs through well-defined interfaces. Thus, modularity and the possibility of parallel work are achieved within a single, easy-to-deploy application.

## **II. Foundation: Database Schema and Data Access Layer**

This section is dedicated to designing the persistence layer, which is the foundation for storing the application's state. A well-designed schema and a clean data access layer are key to the stability, testability, and future development of the system.

### **2.1. Designing the SQLite Database Schema**

The data schema is designed to accurately model the domain, taking into account all functional requirements. It consists of three key tables.

* **users**: Stores information about family members.  
  * id (INTEGER, PRIMARY KEY): Unique user identifier in the system.  
  * telegram\_user\_id (INTEGER, UNIQUE, NOT NULL): Unique user identifier in Telegram. Used to link to the Telegram account.  
  * first\_name (TEXT, NOT NULL): User's first name.  
  * is\_admin (INTEGER, NOT NULL, DEFAULT 0): A flag indicating whether the user is an administrator (1 for yes, 0 for no).  
  * is\_active (INTEGER, NOT NULL, DEFAULT 1): A flag indicating whether the user participates in the duty rotation (1 for yes, 0 for no). Allows for temporarily excluding users from the schedule.  
* **duties**: The main table that stores the duty schedule itself.  
  * id (INTEGER, PRIMARY KEY): Unique identifier for the duty record.  
  * user\_id (INTEGER, NOT NULL): Foreign key referencing users.id.  
  * duty\_date (TEXT, UNIQUE, NOT NULL): The date of the duty in YYYY-MM-DD format. Uniqueness ensures that there can only be one assignment per date.  
  * assignment\_type (TEXT, NOT NULL): The type of assignment. Can be one of three values: 'round\_robin', 'voluntary', 'admin'.  
  * created\_at (TEXT, NOT NULL): Timestamp of the record's creation in ISO 8601 format (UTC).  
* **round\_robin\_state**: An auxiliary but critically important table for implementing a fair round-robin algorithm with load balancing.  
  * user\_id (INTEGER, PRIMARY KEY): Foreign key referencing users.id.  
  * assignment\_count (INTEGER, NOT NULL, DEFAULT 0): A counter for the number of round\_robin assignments for this user.  
  * last\_assigned\_timestamp (TEXT): Timestamp of the last round\_robin assignment in ISO 8601 format (UTC). Used to resolve ties when assignment\_count is the same.

The need for the round\_robin\_state table is driven by the requirement not just for rotation, but for "load balancing." A naive implementation that simply iterates through a list of users in memory is fragile and unfair. Firstly, upon application restart, the rotation state would be lost, potentially leading to the same user being assigned again. Secondly, such an implementation does not account for the total number of duties performed by each participant. Persistently storing the state in round\_robin\_state solves both problems. It ensures that even after a failure or restart, the system continues from where it left off, and it provides long-term fairness by assigning the duty to the person who has served the fewest times.20 This transforms the algorithm from a simple cyclical switch into a reliable and provably fair system.

**Table 2: Database Schema Definitions**

| Table Name | Column Name | Data Type (SQLite) | Constraints | Description |
| :---- | :---- | :---- | :---- | :---- |
| **users** | id | INTEGER | PRIMARY KEY AUTOINCREMENT | Internal unique user identifier. |
|  | telegram\_user\_id | INTEGER | UNIQUE, NOT NULL | User identifier in Telegram. |
|  | first\_name | TEXT | NOT NULL | User's first name. |
|  | is\_admin | INTEGER | NOT NULL, DEFAULT 0 | Administrator flag (1=yes, 0=no). |
|  | is\_active | INTEGER | NOT NULL, DEFAULT 1 | Rotation participation flag (1=yes, 0=no). |
| **duties** | id | INTEGER | PRIMARY KEY AUTOINCREMENT | Unique identifier for the duty record. |
|  | user\_id | INTEGER | NOT NULL, FOREIGN KEY(users.id) | ID of the assigned user. |
|  | duty\_date | TEXT | UNIQUE, NOT NULL | Duty date in 'YYYY-MM-DD' format. |
|  | assignment\_type | TEXT | NOT NULL | Assignment type: 'round\_robin', 'voluntary', 'admin'. |
|  | created\_at | TEXT | NOT NULL | Creation date and time (UTC). |
| **round\_robin\_state** | user\_id | INTEGER | PRIMARY KEY, FOREIGN KEY(users.id) | User ID. |
|  | assignment\_count | INTEGER | NOT NULL, DEFAULT 0 | Counter for 'round\_robin' assignments. |
|  | last\_assigned\_timestamp | TEXT | NULL | Date and time of the last 'round\_robin' assignment (UTC). |

### **2.2. Data Access Layer (DAL) — The "Repository" Pattern**

The "Repository" pattern will be implemented for interacting with the database. This approach allows for the complete isolation of business logic from the specific implementation of the data store.

All data access logic will be concentrated in the /internal/store package. Within this package, a Store interface will be defined, describing all necessary data operations in terms of the domain, for example:

Go

// /internal/store/store.go  
package store

type Store interface {  
    // User methods  
    GetUserByTelegramID(ctx context.Context, id int64) (\*User, error)  
    ListActiveUsers(ctx context.Context) (\*User, error)  
      
    // Duty methods  
    CreateDuty(ctx context.Context, duty \*Duty) error  
    GetDutyByDate(ctx context.Context, date string) (\*Duty, error)  
    UpdateDuty(ctx context.Context, duty \*Duty) error  
      
    // Round Robin methods  
    GetNextRoundRobinUser(ctx context.Context) (\*User, error)  
    IncrementAssignmentCount(ctx context.Context, userID int) error  
}

The concrete implementation of this interface for SQLite will be located in a subpackage, for example, /internal/store/sqlite. This implementation will use the CGo-free modernc.org/sqlite driver to execute SQL queries.2

The advantages of this approach are:

1. **Testability:** In unit tests for business logic, the real Store implementation can be easily replaced with a mock object, allowing the logic to be tested in complete isolation from the database.  
2. **Flexibility:** If the need arises in the future to switch from SQLite to another DBMS (e.g., PostgreSQL), only a new implementation of the Store interface (e.g., /internal/store/postgres) will need to be created, without making changes to the main application business logic.  
3. **Separation of Concerns:** Business logic developers operate with high-level interface methods, without thinking about the details of SQL queries and transaction management.

## **III. Core Engine: Scheduling Logic and Business Rules**

This section describes the "brain" of the application—the business logic that determines how duties are assigned. This logic will be encapsulated in a separate, easily testable module, completely isolated from the delivery mechanisms (Telegram, HTTP).

### **3.1. The scheduler Module**

All core logic will be located in the /internal/scheduler package. This module will depend only on the store.Store interface defined in the previous section, which adheres to the dependency inversion principle. It will have no knowledge of Telegram or HTTP; its sole task is to implement the rules for assigning duties.

### **3.2. Implementation of Assignment Algorithms**

The system supports three types of assignments, which form a strict hierarchy of priorities: Administrative \> Voluntary \> Round-Robin. This hierarchy must be explicitly modeled in the business logic to prevent conflicts and ensure predictable system behavior. Any duty assignment operation should be treated as a transaction that changes the system's state, taking these rules into account.

* **Fair Round-Robin:**  
  * This is the primary mechanism for automatic assignment. The algorithm will query the storage via the store.GetNextRoundRobinUser() method.  
  * The implementation of this method in the data layer will query the round\_robin\_state table to find an active user (is\_active \= 1\) with the minimum assignment\_count value.  
  * In case multiple users have the same minimum number of assignments, the one with the oldest last\_assigned\_timestamp (or NULL) will be chosen to resolve the "tie."  
  * This approach ensures long-term fairness and balancing, as it does not just cycle through users but actively seeks to equalize the total number of duties among all participants.22  
  * After a successful round\_robin assignment, the store.IncrementAssignmentCount() method will be called to update the state of the assigned user.  
* **Voluntary Assignment:**  
  * A user can volunteer for duty on a specific date.  
  * The logic for handling this operation must first check if that date is occupied by an administrative assignment.  
  * If an administrative assignment already exists for the specified date, the request is rejected, as administrative assignments have the highest priority.  
  * If the date is free or occupied by a round\_robin or another voluntary assignment, the new assignment is accepted, and the old one (if it existed) is removed.  
  * Important: A voluntary assignment **must not** change the assignment\_count in the round\_robin\_state table, as it is not part of the fair rotation.  
* **Administrative Assignment:**  
  * This is the assignment with the highest priority. An administrator can assign any user to any date.  
  * This operation overwrites any existing assignment for that date, regardless of its type.  
  * Critically, as per the requirements, an administrative assignment **is not counted in the balancing**. Therefore, the logic for this assignment **must not** trigger an increment of assignment\_count in the round\_robin\_state table. This preserves the integrity and fairness of the automatic scheduling system, allowing the administrator to intervene in the schedule without disrupting the long-term balance.

Modeling this process as a finite state machine with clear transition rules between states (free date, date with a round\_robin assignment, date with an admin assignment, etc.) is key to creating a reliable and predictable system.

## **IV. Service Implementation: Backend Components in GoLang**

This section details the implementation of the application's entry points: the Telegram bot, the web API, and the notification scheduler. These components will serve as "thin" wrappers around the core scheduler and store modules, coordinating interaction with the outside world.

### **4.1. Telegram Bot Service (/internal/telegram)**

This module, using the go-telegram-bot-api/v5 library 1, will be responsible for all logic related to interacting with the Telegram API. It will be launched in a separate goroutine from

main.go and will listen for incoming updates.

* **Command Handlers:**  
  * /start and /help: Display a welcome message and a list of available commands.  
  * /schedule: Shows the current duty schedule for the current month using an interactive inline keyboard for navigating through months.  
  * /volunteer: Allows a user to select a free date from the calendar to sign up for duty.  
  * /status: Shows duty statistics for the current user.  
* **Callback Query Handling:** For processing button presses on inline keyboards (e.g., "previous/next month" in the calendar).  
* **Admin Commands:**  
  * /assign \<username\> \<date\>: Assigns the specified user to duty on the specified date.  
  * /modify \<date\> \<new\_username\>: Changes the person on duty for the specified date.  
  * /users: Shows a list of all registered users with their statuses (active/inactive, admin).  
  * /toggle\_active \<username\>: Enables or disables a user's participation in the rotation.

### **4.2. Web API Service (/internal/http)**

This module, built on the Gin framework 4, will provide a RESTful API for the frontend application. It will also be launched in a separate goroutine.

* **Endpoints:** The API will be structured to provide the data needed by the calendar and the administrative panel.  
* **Authentication and Authorization:** The system will support two levels of access: public read-only and authenticated.  
  * **Public Access:** Unauthenticated users can view the schedule.  
  * **Authenticated Access:** Users accessing the site through the Telegram Web App will be authenticated. The frontend will send the initData string provided by Telegram in the Authorization: tma \<initData\> header.34  
  * A Gin middleware will intercept all requests to protected endpoints. This middleware will use the initdata.Validate() function from the github.com/telegram-mini-apps/init-data-golang library to verify the data against the bot's token.35  
  * If the data is valid, the parsed user information is added to the request's context. Subsequent handlers will check the is\_admin flag for administrative actions.  
  * If validation fails, a 401 Unauthorized status is returned.

**Table 3: REST API Endpoint Specification**

| Endpoint | Method | URL | Access Level | Request Body (JSON) | Success Response (JSON) |
| :---- | :---- | :---- | :---- | :---- | :---- |
| **Get Schedule** | GET | /api/v1/schedule/:year/:month | Public | \- | 200 OK: \`\` |
| **Get Users** | GET | /api/v1/users | Public | \- | 200 OK: \[{ "id": 1, "name": "...", "is\_active": true, "is\_admin": false }\] |
| **Volunteer for Duty** | POST | /api/v1/duties/volunteer | Authenticated | { "date": "YYYY-MM-DD" } | 201 Created |
| **Admin: Assign Duty** | POST | /api/v1/duties | Admin | { "user\_id": 1, "date": "YYYY-MM-DD" } | 201 Created |
| **Admin: Modify Duty** | PUT | /api/v1/duties/:date | Admin | { "user\_id": 2 } | 200 OK |
| **Admin: Delete Duty** | DELETE | /api/v1/duties/:date | Admin | \- | 204 No Content |

This specification serves as a formal contract between backend and frontend developers, allowing for parallel development.

### **4.3. Notification Service (/internal/notification)**

This module is responsible for implementing one of the key features of the system—daily notifications.

* **Scheduler Initialization:** In main.go, an instance of the scheduler will be created using cron.New(). A crucial step is specifying the correct time zone. The requirement "at 16:00 Berlin time" means the system must correctly handle daylight saving time changes. The robfig/cron/v3 library elegantly solves this problem using the CRON\_TZ prefix.8  
  Go  
  // main.go  
  c := cron.New(cron.WithLocation(time.LoadLocation("Europe/Berlin")))  
  // or using CRON\_TZ in the string  
  c.AddFunc("CRON\_TZ=Europe/Berlin 0 16 \* \* \*", dailyNotificationFunc)  
  c.Start()

* **Notification Logic:** The function triggered by the schedule will perform the following actions:  
  1. Determine tomorrow's date.  
  2. Check via store.GetDutyByDate() if a duty is scheduled for tomorrow.  
  3. If a duty is **not scheduled**, call a method from the scheduler module to automatically assign a person on duty using the round-robin algorithm.  
  4. Once the person on duty for tomorrow is determined (either pre-assigned or just automatically assigned), format a message and send it to the family chat via the Telegram API.

It is critically important to adhere to the practice of storing all timestamps in the database in UTC format. Conversion to local time (Europe/Berlin) should only occur at the moment of processing logic related to the schedule or for display to the user. This approach avoids many common errors related to time zones and daylight saving time.

## **V. User Interface: A Modern Frontend with Vanilla JS and Tailwind CSS**

This section describes the strategy for creating a lightweight, mobile-responsive web interface. The main focus is on modern JavaScript practices without involving heavy frameworks, which aligns with the technical specification.

### **5.1. Setting Up the Frontend Development Environment**

The entire frontend part will be located in the /web directory. The structure will be as follows:

* /web/index.html: The main HTML file of the application.  
* /web/js/: Directory for JavaScript modules.  
* /web/css/: Directory for source CSS files.  
* /web/assets/: Directory for static resources (images, icons).

To work with Tailwind CSS, its Standalone CLI version will be used.13 A script will be added to the

package.json file in the project root to run the CLI in watch mode (--watch). This process will scan all .html and .js files for the use of Tailwind utility classes and automatically compile them into a single file /web/dist/output.css.13 This provides a convenient development process without the need to manually recompile styles constantly.

### **5.2. Modular Architecture with Vanilla JS**

Despite the absence of a framework, the frontend application will have a strict and maintainable structure based on native ES6 modules (import/export).11 This approach allows breaking the code into logical, reusable components, which is a key factor in preventing the code from turning into "spaghetti code".23

* js/main.js: The entry point of the application. Initializes the main components and starts rendering the calendar.  
* js/api.js: A module that encapsulates all interaction with the backend API. It will contain functions for making fetch requests to the endpoints defined in Table 3, handling responses, and managing authentication headers. It will retrieve initData from the Telegram Web App context and include it in the Authorization header for authenticated requests.34  
* js/store.js: A simple object for managing client-side state. It will store the current schedule data, the selected month, the list of users, and the authenticated user's state.  
* js/ui/calendar.js: The main UI module responsible for displaying the calendar, handling user interactions (clicks on dates, switching months), and displaying duty information. It will conditionally render interactive elements based on the user's authentication status.

### **5.3. Calendar Component Integration**

Creating a full-featured calendar from scratch is a time-consuming task. To speed up development and achieve a quality result, a lightweight, dependency-free JavaScript calendar library will be used. An excellent candidate is **Vanilla Calendar Pro** 25, which provides rich functionality, theme support, localization, and does not require external libraries. Alternatives like Calendarify 27 or vanillajs-datepicker 28 could also be considered.

The chosen library will be integrated into the js/ui/calendar.js module. Upon initialization and when the month changes, the calendar will call functions from js/api.js to get the latest duty data and then display it in the appropriate cells. The interface will be rendered in a read-only mode for all visitors. If the application detects it is running inside the Telegram Web App and a user is authenticated, additional interactive elements (e.g., buttons to volunteer or admin controls) will be enabled.

This architecture allows for the creation of a clean, structured, and easily maintainable frontend application using only the native capabilities of modern JavaScript, avoiding the excessive complexity and dependencies that large frameworks introduce.

## **VI. Operational Readiness: Containerization, Deployment, and Configuration**

This section describes the final steps for packaging the application into a production-ready container and deploying it reliably on the server.

### **6.1. Multi-Stage Dockerfile**

A multi-stage Dockerfile, located in the /deployments directory, will be used to build the application. This approach is the gold standard for creating optimized and secure images for Go applications.

* **Stage 1 (builder):**  
  * Base image: golang:1.22-alpine. The Alpine version is chosen for its small size.  
  * At this stage, the application's source code (go.mod, go.sum, and the entire project) is copied.  
  * Dependencies are downloaded (go mod download).  
  * The application is compiled with flags that create a statically linked binary and disable debug information. The choice of a CGo-free SQLite driver plays a key role here, as it does not require installing gcc and build-base in this build container.  
  * Frontend resources are also built at this stage (running npm install and npm run build to compile Tailwind CSS).  
* **Stage 2 (final):**  
  * Base image: scratch or gcr.io/distroless/static. scratch is an absolutely empty image containing nothing but the application itself. distroless is an image from Google that contains only the minimally necessary libraries for the application to run and does not include a shell or package manager. Both options significantly reduce the attack surface.  
  * At this stage, only two artifacts are copied from the builder stage:  
    1. The compiled application binary.  
    2. The directory with static frontend resources (/web/dist).  
  * The ENTRYPOINT is set to run the binary file.

The result is an extremely small (typically 10-20 MB) and secure Docker image containing only what is necessary for the application to run.

### **6.2. Orchestration with Docker Compose**

A docker-compose.yml file in the /deployments directory will be used to manage deployment on the server.

YAML

version: '3.8'

services:  
  roster-bot:  
    image: ghcr.io/your-username/roster-bot:latest  
    container\_name: roster-bot  
    restart: always  
    ports:  
      \- "8080:8080"  
    volumes:  
      \- roster-data:/app/data  
    environment:  
      \- TELEGRAM\_APITOKEN=${TELEGRAM\_APITOKEN}  
      \- GIN\_MODE=release  
      \# Other environment variables

volumes:  
  roster-data:

Key aspects of this configuration:

* **image**: Points to the image that will be downloaded from the GitHub Container Registry.  
* **restart: always**: Ensures that the container will be automatically restarted in case of failure or after a server reboot.  
* **volumes**: This section is critically important. It defines a named volume roster-data and mounts it to the /app/data directory inside the container. The SQLite database file (roster.db) will be stored in this directory. Using a named volume is the best practice for storing persistent data. Unlike a bind mount, it is managed by Docker, is more portable, and does not depend on the host's file system structure. This guarantees that the data (the entire duty schedule) will be preserved even if the container is deleted and recreated.  
* **environment**: The application's configuration (tokens, keys) is passed through environment variables, which aligns with the principles of the Twelve-Factor App. Values can be substituted from an .env file or set directly in the runtime environment.

### **6.3. Server Management with Portainer**

Portainer provides a convenient graphical interface for managing the Docker environment. Deploying the application with it comes down to a few simple steps:

1. Go to the "Stacks" section.  
2. Click "Add stack".  
3. Give the stack a name (e.g., roster-bot-stack).  
4. Paste the contents of the docker-compose.yml file into the web editor.  
5. In the "Environment variables" section, securely specify the values for TELEGRAM\_APITOKEN and other secrets.  
6. Click "Deploy the stack".

Portainer will handle the creation of all necessary resources (container, volume) and their launch.

## **VII. Automation at Scale: CI/CD Pipeline with GitHub Actions**

Automating the build, testing, and deployment process is an integral part of modern software development. For this project, a CI/CD pipeline will be configured using GitHub Actions.

### **7.1. Defining the Workflow (.github/workflows/ci-cd.yml)**

A file .github/workflows/ci-cd.yml will be created in the project root, describing the entire process.

* **Trigger:** The workflow will be triggered automatically on every push to the main branch. This ensures that only tested and successfully built code is present in the image repository.  
* **Jobs:**  
  1. **test**:  
     * Runs on ubuntu-latest.  
     * Uses actions/checkout to get the code.  
     * Sets up the Go environment (actions/setup-go).  
     * Runs all unit tests in the project with the command go test./....  
     * This job is a mandatory quality control step.  
  2. **build-and-push**:  
     * This job depends on the successful completion of the test job (needs: test).  
     * Uses docker/setup-qemu-action and docker/setup-buildx-action to configure advanced Docker build capabilities, which is a good practice.  
     * **Authentication to GHCR:** The docker/login-action is used to log in to the GitHub Container Registry. The key here is the use of GITHUB\_TOKEN. Instead of creating a long-lived Personal Access Token (PAT) 29, a temporary token that GitHub Actions generates for each workflow run is used. The necessary permissions (  
       write for packages) are granted at the job level using the permissions block, which is a modern and secure practice.16  
     * **Metadata Generation:** The docker/metadata-action is used to automatically create tags for the Docker image. For example, latest for the last commit in main and a tag with the short commit hash for versioning.  
     * **Build and Push:** The docker/build-push-action performs the image build using the multi-stage Dockerfile and pushes it to GHCR with the generated tags.30

Example workflow configuration:

YAML

\#.github/workflows/ci-cd.yml  
name: CI/CD Pipeline

on:  
  push:  
    branches: \[ "main" \]

jobs:  
  test:  
    runs-on: ubuntu-latest  
    steps:  
      \- uses: actions/checkout@v4  
      \- name: Set up Go  
        uses: actions/setup-go@v5  
        with:  
          go-version: '1.22'  
      \- name: Run tests  
        run: go test \-v./...

  build-and-push:  
    runs-on: ubuntu-latest  
    needs: test  
    permissions:  
      contents: read  
      packages: write \# Required for pushing to GHCR  
    steps:  
      \- name: Checkout repository  
        uses: actions/checkout@v4

      \- name: Log in to the Container registry  
        uses: docker/login-action@v3  
        with:  
          registry: ghcr.io  
          username: ${{ github.actor }}  
          password: ${{ secrets.GITHUB\_TOKEN }}

      \- name: Extract metadata (tags, labels) for Docker  
        id: meta  
        uses: docker/metadata-action@v5  
        with:  
          images: ghcr.io/${{ github.repository }}

      \- name: Build and push Docker image  
        uses: docker/build-push-action@v6  
        with:  
          context:.  
          file:./deployments/Dockerfile  
          push: true  
          tags: ${{ steps.meta.outputs.tags }}  
          labels: ${{ steps.meta.outputs.labels }}

This pipeline fully automates the application delivery process from commit to a ready-to-deploy Docker image, ensuring high development speed and reliability.

## **VIII. Future Horizons: LLM Integration Plan**

The final section discusses the strategic plan for integrating a large language model (LLM), as mentioned in the requirements. The architecture is initially designed so that this functionality can be added in the future with minimal effort.

### **8.1. Designing an Extensible llm Service**

The key to a flexible and durable integration is the application of the **"Adapter" pattern**. Instead of tightly coupling the code to a specific provider's API (e.g., OpenAI), an internal abstract interface will be created.

A new package /internal/llm will be created in the project, containing a Provider interface:

Go

// /internal/llm/provider.go  
package llm

type Provider interface {  
    // Query sends a request to the LLM and returns the response as a string.  
    Query(ctx context.Context, prompt string) (string, error)  
}

This interface defines a common contract for interacting with any LLM. Other parts of the application (e.g., the Telegram command handler) will depend only on this interface, not on a specific implementation.

### **8.2. Implementation with an OpenAI-Compatible Client**

To implement the initial requirement, a concrete adapter satisfying the llm.Provider interface will be created.

* **openai.Provider Implementation**: This component will be located in the /internal/llm/openai package. It will use an official or popular Go library for working with the OpenAI API, such as github.com/openai/openai-go 31 or  
  github.com/sashabaranov/go-openai.32  
* **Configuration**: The client will be configured via environment variables, accepting an API key and, importantly, an endpoint URL. This makes it compatible not only with OpenAI but also with any other service that provides an OpenAI-compatible API (e.g., locally deployed models via Ollama or vLLM).

This approach completely isolates the main application logic from the implementation details of a specific LLM provider. If in the future it becomes necessary to switch to another model (e.g., Anthropic Claude or Google Gemini), it will only be necessary to write a new adapter (e.g., anthropic.Provider) without changing the code that uses it.

### **8.3. Potential Use Cases**

LLM integration opens up possibilities for implementing more "intelligent" features.

* **New /ask \<question\> command in Telegram:**  
  * Upon receiving this command, the bot first retrieves the current and past duty schedules from the database.  
  * This data is formatted as context (e.g., in Markdown or JSON format).  
  * A prompt for the LLM is formed, which includes this context and the user's question.  
  * Example prompts:  
    * "Based on the following duty data: \[...data...\]. Answer the question: Who is on duty tomorrow?"  
    * "Based on the following duty data: \[...data...\]. Answer the question: When was Ivan last on duty?"  
  * The LLM processes the prompt, extracts the necessary information from the provided context, and generates a natural language response, which the bot then sends to the user.

This approach allows users to interact with the system in a more natural and conversational manner, not limited to strict commands.