package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kartoza/go-timesheets-go/internal/service"
	"github.com/kartoza/go-timesheets-go/internal/storage"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	dataDir := filepath.Join(homeDir, ".kartoza-timesheet")
	storage, err := storage.New(dataDir)
	if err != nil {
		panic(err)
	}

	service := service.New(storage, "default-user")

	// Create sample projects
	projects := []struct {
		name, description string
	}{
		{"WB GEEST 2 Enhancements", "World Bank GEEST 2 system enhancements and improvements"},
		{"WB - Housing Planning Scenario Explorer", "World Bank housing planning scenario explorer development"},
		{"QGIS Consultant Contract", "QGIS consultation and development work"},
		{"Planet Labs - Support", "Planet Labs technical support and integration"},
		{"Kartoza Sales", "Sales activities and client engagement"},
	}

	for _, p := range projects {
		project, err := service.CreateProject(p.name, p.description)
		if err != nil {
			fmt.Printf("Error creating project %s: %v\n", p.name, err)
			continue
		}

		// Create sample tasks for each project
		tasks := []struct {
			name        string
			description string
			expected    float64
		}{
			{"Task 1: Initial Setup", "Initial project setup and configuration", 8.0},
			{"Task 2: Core Development", "Main development work", 40.0},
			{"Task 3: Improved Functionalities", "Enhancement and improvement implementation", 24.0},
			{"Planning and setup", "Project planning and environment setup", 16.0},
		}

		for _, t := range tasks {
			_, err := service.CreateTask(project.ID, t.name, t.description, t.expected)
			if err != nil {
				fmt.Printf("Error creating task %s: %v\n", t.name, err)
			}
		}

		fmt.Printf("✅ Created project: %s\n", project.Name)
	}

	fmt.Println("\n🎉 Sample data setup complete!")
	fmt.Println("You can now run: kartoza-timesheet")
}
