package workers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	log "github.com/sirupsen/logrus"

	"math"
	"os"
	"path/filepath"

	"strings"

	"github.com/google/uuid"

	app "github.com/appdynamics/cluster-agent/appd"
	m "github.com/appdynamics/cluster-agent/models"
	"github.com/appdynamics/cluster-agent/utils"
)

const (
	WIDGET_GAP      float64 = 10
	FULL_CLUSTER    string  = "/cluster-template.json"
	CLUSTER_WIDGETS string  = "/cluster-widgets.json"
	DEPLOY_BASE     string  = "/base_deploy_template.json"
	TIER_SUMMARY    string  = "/tier_stats_widget.json"
	BACKGROUND      string  = "/background.json"
	HEAT_MAP        string  = "/heatnodetemplate.json"
	HEAT_WIDGET     string  = "/healthwidget.json"
)

type DashboardWorker struct {
	Bag            *m.AppDBag
	Logger         *log.Logger
	AppdController *app.ControllerClient
	AdqlWorker     AdqlSearchWorker
}

func NewDashboardWorker(bag *m.AppDBag, l *log.Logger, appController *app.ControllerClient) DashboardWorker {
	dw := DashboardWorker{bag, l, appController, NewAdqlSearchWorker(bag, l)}
	return dw
}

func (dw *DashboardWorker) ensureClusterDashboard() error {
	dashName := fmt.Sprintf("%s-%s-%s", dw.Bag.AppName, dw.Bag.TierName, dw.Bag.DashboardSuffix)
	dashboard, err := dw.loadDashboard(dashName)
	if err != nil {
		return err
	}
	if dashboard == nil {
		fmt.Printf("Dashboard %s does not exist. Creating...\n", dashName)
		dw.createClusterDashboard(dashName)
	}
	return nil
}

func (dw *DashboardWorker) createDashboard(dashboard *m.Dashboard) (*m.Dashboard, error) {
	var dashObject *m.Dashboard = nil
	data, err := json.Marshal(dashboard)
	if err != nil {
		return dashObject, fmt.Errorf("Unable to create dashboard %s. %v", dashboard.Name, err)
	}
	rc := app.NewRestClient(dw.Bag, dw.Logger)
	saved, errSave := rc.CallAppDController("restui/dashboards/createDashboard", "POST", data)
	if errSave != nil {

		return dashObject, fmt.Errorf("Unable to create dashboard %s. %v\n", dashboard.Name, errSave)
	}

	e := json.Unmarshal(saved, &dashObject)
	if e != nil {
		fmt.Printf("Unable to deserialize the new dashboard %v\n", e)
		return dashObject, e
	}

	return dashObject, nil
}

func (dw *DashboardWorker) saveDashboard(dashboard *m.Dashboard) (*m.Dashboard, error) {
	var dashObject *m.Dashboard = nil
	data, err := json.Marshal(dashboard)
	if err != nil {
		return nil, fmt.Errorf("Unable to save dashboard %s. %v", dashboard.Name, err)
	}
	rc := app.NewRestClient(dw.Bag, dw.Logger)
	saved, errSave := rc.CallAppDController("restui/dashboards/updateDashboard", "POST", data)
	if errSave != nil {
		return nil, fmt.Errorf("Unable to save dashboard %s. %v\n", dashboard.Name, errSave)
	}

	e := json.Unmarshal(saved, &dashObject)
	if e != nil {
		fmt.Printf("Unable to deserialize the saved dashboard %v\n", e)
		return nil, e
	}

	return dashObject, nil
}

func (dw *DashboardWorker) updateWidget(widget *map[string]interface{}) error {
	data, err := json.Marshal(widget)
	if err != nil {
		return fmt.Errorf("Unable to save widget %s. %v", (*widget)["type"], err)
	}
	rc := app.NewRestClient(dw.Bag, dw.Logger)
	_, errSave := rc.CallAppDController("restui/dashboards/updateWidget", "POST", data)
	if errSave != nil {
		return fmt.Errorf("Unable to update widget %s. %v\n", (*widget)["type"], errSave)
	}

	return nil
}

func (dw *DashboardWorker) loadDashboardByID(id float64) (*m.Dashboard, error) {
	var theDash m.Dashboard

	fmt.Println("Checking the dashboard...")
	rc := app.NewRestClient(dw.Bag, dw.Logger)
	data, err := rc.CallAppDController(fmt.Sprintf("restui/dashboards/dashboardIfUpdated/%.0f/-1", id), "GET", nil)
	if err != nil {
		fmt.Printf("Unable to get the dashboard by id %.0f. %v\n", id, err)
		return nil, err
	}

	e := json.Unmarshal(data, &theDash)
	if e != nil {
		fmt.Printf("Unable to deserialize the dashboards. %v\n", e)
		return nil, e
	}

	return &theDash, nil
}

func (dw *DashboardWorker) loadDashboard(dashName string) (*m.Dashboard, error) {
	var theDash *m.Dashboard = nil

	fmt.Println("Checking the dashboard...")
	rc := app.NewRestClient(dw.Bag, dw.Logger)
	data, err := rc.CallAppDController("restui/dashboards/getAllDashboardsByType/false", "GET", nil)
	if err != nil {
		fmt.Printf("Unable to get the list of dashboard. %v\n", err)
		return theDash, err
	}
	var list []m.Dashboard
	e := json.Unmarshal(data, &list)
	if e != nil {
		fmt.Printf("Unable to deserialize the list of dashboards. %v\n", e)
		return theDash, e
	}

	for _, d := range list {
		if d.Name == dashName {
			fmt.Printf("Dashboard %s exists.\n", dashName)
			theDash = &d
			break
		}
	}
	return theDash, nil
}

func (dw *DashboardWorker) readJsonTemplate(templateFile string) ([]byte, error, bool) {
	if templateFile == "" {
		templateFile = dw.Bag.DashboardTemplatePath
	}

	dir := filepath.Dir(dw.Bag.DashboardTemplatePath)
	templateFile = dir + templateFile

	exists := true

	jsonFile, err := os.Open(templateFile)

	if err != nil {
		exists = !os.IsNotExist(err)
		dw.Logger.Warnf("Unable to open template file %s. %v", templateFile, err)
		return nil, fmt.Errorf("Unable to open template file %s. %v", templateFile, err), exists
	}
	dw.Logger.Debugf("Successfully opened template %s\n", templateFile)
	defer jsonFile.Close()

	byteValue, errRead := ioutil.ReadAll(jsonFile)
	if errRead != nil {
		dw.Logger.Errorf("Unable to read json file %s. %v", templateFile, errRead)
		return nil, fmt.Errorf("Unable to read json file %s. %v", templateFile, errRead), exists
	}

	return byteValue, nil, exists
}

func (dw *DashboardWorker) loadDashboardTemplate(templateFile string) (*m.Dashboard, error, bool) {
	var dashObject *m.Dashboard = nil
	byteValue, err, exists := dw.readJsonTemplate(templateFile)
	if err != nil || !exists {
		return nil, err, exists
	}

	serErr := json.Unmarshal([]byte(byteValue), &dashObject)
	if serErr != nil {
		return dashObject, fmt.Errorf("Unable to get dashboard template %s. %v", templateFile, serErr), exists
	}

	return dashObject, nil, exists
}

func (dw *DashboardWorker) loadWidgetTemplate(templateFile string) (*[]map[string]interface{}, error, bool) {
	var widget *[]map[string]interface{} = nil

	byteValue, err, exists := dw.readJsonTemplate(templateFile)
	if err != nil || !exists {
		return nil, err, exists
	}

	serErr := json.Unmarshal([]byte(byteValue), &widget)
	if serErr != nil {
		return widget, fmt.Errorf("Unable to get widget template %s. %v", templateFile, serErr), exists
	}

	return widget, nil, exists
}

func (dw *DashboardWorker) createClusterDashboard(dashName string) error {

	dashObject, err, _ := dw.loadDashboardTemplate(FULL_CLUSTER)
	if err != nil {
		return err
	}

	dashObject.Name = dashName

	metricsMetadata := m.NewClusterPodMetricsMetadata(dw.Bag, m.ALL, m.ALL).Metadata
	dw.updateDashboard(dashObject, metricsMetadata)

	_, errCreate := dw.createDashboardFromFile(dashObject, "/GENERATED.json")

	return errCreate
}

func (dw *DashboardWorker) createDashboardFromFile(dashboard *m.Dashboard, genPath string) (*m.Dashboard, error) {

	fileForUpload, err := dw.saveTemplate(dashboard, genPath)
	if err != nil {
		return nil, fmt.Errorf("Unable to create dashboard. %v\n", err)
	}

	rc := app.NewRestClient(dw.Bag, dw.Logger)
	data, errCreate := rc.CreateDashboard(*fileForUpload)
	if errCreate != nil {
		return nil, errCreate
	}
	var raw map[string]interface{}
	e := json.Unmarshal(data, &raw)
	if e != nil {
		return nil, fmt.Errorf("Unable to create dashboard. Unable to deserialize the new dashboards. %v\n", err)
	}
	bytes, errM := json.Marshal(raw)
	if errM != nil {
		return nil, fmt.Errorf("Unable to extract dashboard node. %v\n", errM)
	}
	var dashObj m.Dashboard
	eD := json.Unmarshal(bytes, &dashObj)
	if eD != nil {
		return nil, fmt.Errorf("Unable to create dashboard. Unable to deserialize the new dashboards. %v\n", eD)
	}

	dw.Logger.Debugf("Dashboard saved %.0f %s\n", dashObj.ID, dashObj.Name)
	return dashboard, nil
}

func (dw *DashboardWorker) saveTemplate(dashboard *m.Dashboard, genPath string) (*string, error) {
	generated, errMar := json.Marshal(dashboard)
	if errMar != nil {
		return nil, fmt.Errorf("Unable to save template. %v", errMar)
	}
	dir := filepath.Dir(dw.Bag.DashboardTemplatePath)
	fileForUpload := dir + genPath
	dw.Logger.Debugf("About to save template file %s...\n", fileForUpload)
	errSave := ioutil.WriteFile(fileForUpload, generated, 0644)
	if errSave != nil {
		return nil, fmt.Errorf("Issues when writing generated template. %v", errSave)
	}
	dw.Logger.Debugf("File %s... saved\n", fileForUpload)
	return &fileForUpload, nil
}

func (dw *DashboardWorker) updateDashboard(dashObject *m.Dashboard, metricsMetadata map[string]m.AppDMetricMetadata) {
	for _, w := range dashObject.Widgets {
		widget := w
		dsTemplates, ok := widget["dataSeriesTemplates"]
		if ok {
			for _, t := range dsTemplates.(map[string]interface{}) {
				mmTemplate, ok := t.(map[string]interface{})["metricMatchCriteriaTemplate"]
				if ok {
					exp, ok := mmTemplate.(map[string]interface{})["metricExpressionTemplate"]
					if ok {
						dw.updateMetricNode(exp.(map[string]interface{}), metricsMetadata, &widget)
					}
				}
			}
		}
	}
}

func (dw *DashboardWorker) updateMetricNode(expTemplate map[string]interface{}, metricsMetadata map[string]m.AppDMetricMetadata, parentWidget *map[string]interface{}) {
	metricObj := dw.getMatchingMetrics(expTemplate, metricsMetadata)
	if metricObj != nil {
		//metrics path matches. update
		dw.updateMetricPath(expTemplate, metricObj, parentWidget)
	} else {
		dw.updateMetricExpression(expTemplate, metricsMetadata, parentWidget)
	}
}

func (dw *DashboardWorker) updateMetricPath(expTemplate map[string]interface{}, metricObj *m.AppDMetricMetadata, parentWidget *map[string]interface{}) {
	expTemplate["metricPath"] = fmt.Sprintf(metricObj.Path, dw.Bag.TierName)
	scopeBlock := expTemplate["scopeEntity"].(map[string]interface{})
	scopeBlock["applicationName"] = dw.Bag.AppName
	scopeBlock["entityName"] = dw.Bag.TierName

}

func (dw *DashboardWorker) updateMetricExpression(expTemplate map[string]interface{}, metricsMetadata map[string]m.AppDMetricMetadata, parentWidget *map[string]interface{}) {
	for i := 0; i < 2; i++ {
		expNodeName := fmt.Sprintf("expression%d", i+1)
		if expNode, ok := expTemplate[expNodeName]; ok {
			metricObj := dw.getMatchingMetrics(expNode.(map[string]interface{}), metricsMetadata)
			if metricObj != nil {
				dw.updateMetricPath(expNode.(map[string]interface{}), metricObj, parentWidget)
			} else {
				dw.updateMetricExpression(expTemplate, metricsMetadata, parentWidget)
			}
		}

	}
}

func (dw *DashboardWorker) getMatchingMetrics(node map[string]interface{}, metricsMetadata map[string]m.AppDMetricMetadata) *m.AppDMetricMetadata {

	if val, ok := node["metricPath"]; ok {
		if metricObj, matches := metricsMetadata[val.(string)]; matches {
			return &metricObj
		}
	}
	return nil
}

func (dw *DashboardWorker) validateTierDashboardBag(bag *m.DashboardBag) error {
	msg := ""
	if bag.ClusterAppID == 0 {
		msg = "Missing Agent App ID."
	}
	if bag.AppID == 0 {
		msg += " APM App ID is missing."
	}
	if bag.TierID == 0 {
		msg += " APM Tier ID is missing."
	}
	if bag.NodeID == 0 {
		msg += " APM Node ID is missing."
	}
	if msg != "" {
		return fmt.Errorf("Dashboard validation failed. %s", msg)
	}
	return nil
}

//cluster dashboard
func (dw *DashboardWorker) updateClusterOverview(bag *m.DashboardBag) error {
	fileName := fmt.Sprintf("/deploy%s", FULL_CLUSTER)
	dashboard, err, exists := dw.loadDashboardTemplate(fileName)
	if err != nil && exists {
		return fmt.Errorf("Cluster template exists, but cannot be loaded. %v", err)
	}

	dashName := fmt.Sprintf("Cluster-Overview-%s-%s", dw.Bag.AppName, dw.Bag.DashboardSuffix)

	//	if dashboard != nil {
	//		// if the dashboard was created earlier, make sure it still exists
	//		existingDash, _ := dw.loadDashboard(dashName)
	//		//if the dashboard got deleted or exists with a different ID, force to create a new one
	//		if existingDash == nil || dashboard.ID != existingDash.ID {
	//			dashboard = nil
	//		}
	//	}

	if dashboard == nil {
		dw.Logger.Info("Saved template not found. Creating cluster overview dashboard from scratch\n")
		dashboard, err, exists = dw.loadDashboardTemplate(FULL_CLUSTER)
		dw.Logger.Debugf("Load template %s. Error: %v, Exists: %t\n", FULL_CLUSTER, err, exists)
		if !exists {
			return fmt.Errorf("Cluster overview template not found. Aborting dashboard generation...")
		}
		if err != nil {
			return fmt.Errorf("Cluster overview template exists, but cannot be loaded. %v", err)
		}
		dashboard.Name = dashName

		newDash, err := dw.createDashboard(dashboard)
		dw.Logger.Debugf("Create dashboard result. Error: %v, \n", err)
		if err != nil {
			//agent restart, no template left, but the dashboard exists, attempt to load by name
			existing, err := dw.loadDashboard(dashName)
			if err != nil {
				return fmt.Errorf("Unable to load dashboard %s. %v", dashName, err)
			} else {
				dashboard.ID = existing.ID
			}
		} else {
			dashboard = newDash
		}

		//by this time we should have dashboard object with id
		//load the template with all static widgets
		widgetList, err, exists := dw.loadWidgetTemplate(CLUSTER_WIDGETS)
		if err != nil && exists {
			return fmt.Errorf("Cluster widget template exists, but cannot be loaded. %v\n", err)
		}
		if !exists {
			return fmt.Errorf("Cluster widget template does not exist, skipping dashboard. %v\n", err)
		}

		if len(*widgetList) < 2 {
			return fmt.Errorf("Cluster widget template is invalid. Must have at least 2 widget\n")
		}

		dashboard.Widgets = *widgetList

		//replace metric references
		for _, widget := range dashboard.Widgets {
			widget["dashboardId"] = dashboard.ID
			widgetGuid := uuid.New().String()
			widget["guid"] = widgetGuid
			if widget["type"] == "HEALTH_LIST" {
				dw.updateHealthWidget(&widget, bag)
			} else {
				dsTemplates, ok := widget["widgetsMetricMatchCriterias"]
				if ok && dsTemplates != nil {
					for _, t := range dsTemplates.([]interface{}) {
						if t != nil {
							mt := t.(map[string]interface{})
							mt["dashboardId"] = dashboard.ID
							mt["widgetGuid"] = widgetGuid
							mm, ok := mt["metricMatchCriteria"]
							if ok && mm != nil {
								mmBlock := mm.(map[string]interface{})
								mmBlock["applicationId"] = bag.ClusterAppID
								exp, ok := mmBlock["metricExpression"]
								if ok && exp != nil {
									e := dw.updateMetricDefinition(exp.(map[string]interface{}), bag, &widget)
									if e != nil {
										return e
									}
								}
							}
						}
					}
				}
			}
		}

		//update dashboard with static widgets
		savedDash, errSaveDash := dw.saveDashboard(dashboard)

		if errSaveDash != nil {
			dw.Logger.Errorf("Cluster dashboard cannot be saved. %v", errSaveDash)
			return errSaveDash
		}
		dw.Logger.Infof("Cluster dashboard updated.")

		// save template for future use
		_, errSave := dw.saveTemplate(savedDash, fileName)
		if errSave != nil {
			dw.Logger.Errorf("Issues when saving template for Cluster overview dashboard. %v\n", errSave)
		}
		dashboard = savedDash
	}

	if dashboard == nil {
		return fmt.Errorf("Unable to build Cluster overview dashboard. Weird things do happen...")
	}

	//add heat map
	hotdash, heatErr := dw.addPodHeatMap(dashboard, bag)
	if heatErr != nil {
		dw.Logger.Errorf("Unable to add heatmap to the cluster dashboard. %v\n", heatErr)
		return heatErr
	}

	_, errSaveDash := dw.saveDashboard(hotdash)
	if errSaveDash != nil {
		dw.Logger.Errorf("Cluster overview dashboard cannot be saved", errSaveDash)
		return errSaveDash
	}

	dw.Logger.Info("Cluster overview Dashboard saved successfully")

	return nil
}

func (dw *DashboardWorker) addPodHeatMap(dashboard *m.Dashboard, bag *m.DashboardBag) (*m.Dashboard, error) {

	backWidth := 1465
	backTop := 517
	startLine := 540
	backHeight := 159
	minSize := 24
	healthsize := 10
	leftMargin := 26
	nodeMargin := minSize
	backgroundWidetY := 506

	currentX := leftMargin
	currentY := startLine

	width := minSize
	height := minSize

	nodeArray, nsNum, deployNum := bag.GetNodes()

	numPods := len(nodeArray)

	totalUnits := numPods + deployNum + 2*nsNum

	availableArea := (backWidth - 2*nodeMargin) * (backHeight - (startLine - backTop))
	areaPerNode := math.Round(float64(availableArea / totalUnits))
	side := math.Sqrt(areaPerNode)
	if side > float64(minSize+nodeMargin) {
		nodeMargin = int(math.Round(side / 2))
		width = nodeMargin
		height = nodeMargin
	}

	deployGap := int(width)
	nsGap := int(2 * width)

	rightMargin := backWidth - nodeMargin
	lastLine := backTop + backHeight - nodeMargin - height

	backgroundWidet := dashboard.Widgets[1]
	backgroundWidet["dashboardId"] = dashboard.ID
	backgroundWidet["height"] = backHeight
	backgroundWidet["width"] = backWidth
	backgroundWidet["x"] = 14
	backgroundWidet["y"] = backgroundWidetY

	dashBack := dashboard.Widgets[0]
	dashBack["dashboardId"] = dashboard.ID
	dashBack["height"] = 659
	dashBack["width"] = 1479
	dashBack["x"] = 10
	dashBack["y"] = 13

	widgetList, err, exists := dw.loadWidgetTemplate(HEAT_MAP)
	if err != nil && exists {
		return nil, fmt.Errorf("Heatmap template exists, but cannot be loaded. %v\n", err)
	}
	if !exists {
		return nil, fmt.Errorf("Heatmap template does not exist, skipping cluster dashboard. %v\n", err)
	}

	//health widget for APM nodes
	hwList, errHw, existsHw := dw.loadWidgetTemplate(HEAT_WIDGET)
	if errHw != nil && existsHw {
		return nil, fmt.Errorf("Health widget template exists, but cannot be loaded. %v\n", errHw)
	}
	if !existsHw {
		return nil, fmt.Errorf("Health widget template does not exist, skipping cluster dashboard. %v\n", err)
	}

	heatWidget := (*widgetList)[0]
	healthWidget := (*hwList)[0]

	dotArray := []map[string]interface{}{}
	hwdotArray := []map[string]interface{}{}

	oldDeploy := ""
	oldNS := ""
	for _, hn := range nodeArray {
		dot, err := utils.CloneMap(heatWidget)
		if err != nil {
			return nil, fmt.Errorf("Heatmap template is invalid. %v\n", err)
		}
		dot["dashboardId"] = dashboard.ID
		dot["guid"] = uuid.New().String()
		dot["height"] = height
		dot["width"] = width
		contNum := hn.GetContainerCount()
		if contNum > 1 {
			dot["text"] = fmt.Sprintf("%d", len(hn.Containers))
		}
		dot["description"] = fmt.Sprintf("%s/%s\n%s", hn.Namespace, hn.Podname, hn.Nodename)
		dot["description"] = fmt.Sprintf("%s\n%s", dot["description"], hn.GetContainerStatsFormatted())

		if len(hn.Events) > 0 {
			dot["description"] = fmt.Sprintf("%s\n%s", dot["description"], hn.GetEventsFormatted())
		}

		//color
		colorCode := 0 //black
		enableBorder := false
		searchPath := fmt.Sprintf("%s%s", BASE_PATH, "Evictions")
		if hn.State == "Running" || hn.State == "Succeeded" {
			if hn.IsOverconsuming() {
				colorCode = 10040319 //purple
				searchPath = fmt.Sprintf("%s%s", BASE_PATH, "PodOverconsume")
			} else {
				colorCode = 34021 //blue
				searchPath = fmt.Sprintf("%s%s", BASE_PATH, "PodRunning")
			}
			enableBorder = true
		} else if hn.State == "Pending" {
			colorCode = 16605970 //orange
			dot["description"] = fmt.Sprintf("%s\nPending t: %s", dot["description"], hn.FormatPendingTime())
			searchPath = fmt.Sprintf("%s%s", BASE_PATH, "PodPending")
			enableBorder = true
		} else if hn.State == "Failed" {
			colorCode = 13369344 //red
			searchPath = fmt.Sprintf("%s%s", BASE_PATH, "PodFailed")
		}

		dot["backgroundColor"] = colorCode

		if enableBorder && hn.Restarts > 0 {
			dot["borderEnabled"] = true
			dot["borderThickness"] = 2
			dot["borderColor"] = 13369344 //red
			searchPath = fmt.Sprintf("%s%s", BASE_PATH, "PodRestarts")
		}

		if oldDeploy != "" && oldDeploy != hn.Owner {
			currentX += deployGap
		}

		if oldNS != "" && oldNS != hn.Namespace {
			currentX += nsGap
		}
		//revalidate dimensions after the adjustment
		currentX, currentY = dw.validateDimensions(currentX, currentY, lastLine, height, nodeMargin, minSize, rightMargin, leftMargin, dashboard, &backgroundWidet, &dashBack)

		//position
		dot["x"] = currentX
		dot["y"] = currentY

		oldNS = hn.Namespace
		oldDeploy = hn.Owner

		//if APM node add health widget
		if apmID, hasNodeID := hn.GetAPMID(); apmID > 0 {
			healthDot, err := utils.CloneMap(healthWidget)
			if err != nil {
				return nil, fmt.Errorf("Health widget template is invalid. %v\n", err)
			}
			healthDot["dashboardId"] = dashboard.ID
			healthDot["guid"] = uuid.New().String()
			healthDot["height"] = healthsize
			healthDot["width"] = healthsize
			healthDot["iconSize"] = healthsize
			healthDot["applicationId"] = hn.AppID
			healthDot["entityIds"] = []int{apmID}

			//			linkLocation := "APP_COMPONENT_MANAGER"
			//			linkComponent := "component"
			if hasNodeID {
				healthDot["entityType"] = "APPLICATION_COMPONENT_NODE"
				//				linkLocation = "APP_NODE_MANAGER"
				//				linkComponent = "node"
			}

			hwdX := currentX + width - healthsize/2
			hwdY := currentY - healthsize/2

			healthDot["x"] = hwdX
			healthDot["y"] = hwdY
			//			dot["drillDownUrl"] = fmt.Sprintf("%s#/location=%s&timeRange=last_1_hour.BEFORE_NOW.-1.-1.60&application=%d&%s=%d&dashboardMode=force", dw.Bag.RestAPIUrl, linkLocation, hn.AppID, linkComponent, apmID)
			hwdotArray = append(hwdotArray, healthDot)
		}
		//link to pod list by state
		if searchPath != "" {
			searchUrl := dw.AdqlWorker.GetSearch(searchPath)
			if searchUrl != "" {
				dot["drillDownUrl"] = searchUrl
				dot["useMetricBrowserAsDrillDown"] = false
			}
		}
		dotArray = append(dotArray, dot)

		currentX = currentX + minSize + nodeMargin
		currentX, currentY = dw.validateDimensions(currentX, currentY, lastLine, height, nodeMargin, minSize, rightMargin, leftMargin, dashboard, &backgroundWidet, &dashBack)
	}

	//append widgets
	for _, d := range dotArray {
		dashboard.Widgets = append(dashboard.Widgets, d)
	}

	for _, hwd := range hwdotArray {
		dashboard.Widgets = append(dashboard.Widgets, hwd)
	}

	return dashboard, nil
}

func (dw *DashboardWorker) validateDimensions(currentX, currentY, lastLine, height, nodeMargin, minSize, rightMargin, leftMargin int, dashboard *m.Dashboard, backgroundWidet *map[string]interface{}, dashBack *map[string]interface{}) (int, int) {
	if currentX > rightMargin {
		currentX = leftMargin
		currentY = currentY + minSize + nodeMargin
		if currentY > lastLine {
			//make the background taller
			lastLine = lastLine + height + nodeMargin
			(*dashBack)["height"] = currentY + minSize + nodeMargin //lastLine + 2*minSize + 2*nodeMargin
			(*backgroundWidet)["height"] = (*dashBack)["height"].(int) - (*backgroundWidet)["y"].(int)
			if dashboard.Height < float64((*dashBack)["height"].(int)+2*minSize) {
				dashboard.Height = float64((*dashBack)["height"].(int) + 2*minSize)
			}
			dw.Logger.Infof("currentY = %d, height = %d, lastLine = %d", currentY, height, lastLine)
			dw.Logger.Infof("dashBack height = %d", (*dashBack)["height"])
			dw.Logger.Infof("backgroundWidet height = %d", (*backgroundWidet)["height"])
		}
	}
	return currentX, currentY
}

func (dw *DashboardWorker) updateHealthWidget(widget *map[string]interface{}, bag *m.DashboardBag) {
	//determin if the widget is for the agent or a different app
	if (*widget)["label"] == "appd" {
		(*widget)["applicationId"] = bag.ClusterAppID
		(*widget)["entityIds"] = []int{bag.ClusterNodeID}
	} else {
		//specific apps
		(*widget)["applicationId"] = bag.AppID
		if (*widget)["entityType"] == "APPLICATION_COMPONENT_NODE" {
			(*widget)["entityIds"] = []int{bag.NodeID}
		} else {
			(*widget)["entityIds"] = []int{bag.TierID}
		}
	}

}

//dynamic dashboard
func (dw *DashboardWorker) updateTierDashboard(bag *m.DashboardBag) error {
	valErr := dw.validateTierDashboardBag(bag)
	if valErr != nil {
		dw.Logger.Printf("Dashboard parameters are invalid. %v", valErr)
		return valErr
	}

	fileName := fmt.Sprintf("/deploy/%s_%s.json", bag.Namespace, bag.TierName)
	dashboard, err, exists := dw.loadDashboardTemplate(fileName)
	if err != nil && exists {
		return fmt.Errorf("Deployment template for %s/%s exists, but cannot be loaded. %v", bag.Namespace, bag.TierName, err)
	}
	dashName := fmt.Sprintf("%s-%s-%s-%s", dw.Bag.AppName, bag.Namespace, bag.TierName, dw.Bag.DashboardSuffix)
	if !exists {
		fmt.Printf("Checking dashboard for deployment %s/%s on the server\n", bag.Namespace, bag.TierName)
		dashboard, err = dw.loadDashboard(dashName)
		if dashboard == nil {
			fmt.Printf("Not found. Creating dashboard for deployment %s/%s from scratch\n", bag.Namespace, bag.TierName)
			dashboard, err, exists = dw.loadDashboardTemplate(DEPLOY_BASE)
			fmt.Printf("Load template %s. Error: %v, Exists: %t\n", DEPLOY_BASE, err, true)
			if !exists {
				return fmt.Errorf("Deployment template not found. Aborting dashboard generation")
			}
			if err != nil {
				return fmt.Errorf("Deployment template exists, but cannot be loaded. %v", err)
			}
			dashboard.Name = dashName
			dashboard, err = dw.createDashboard(dashboard)
			fmt.Printf("Create dashboard result. Error: %v, \n", err)
			if err != nil {
				return err
			}
		}
	}
	if dashboard == nil {
		return fmt.Errorf("Unable to build tier Dashboard. Weird things do happen...")
	}
	//by this time we should have dashboard object with id
	//save base template with id, strip widgets
	dashboard.Widgets = []map[string]interface{}{}
	_, errSave := dw.saveTemplate(dashboard, fileName)
	if errSave != nil {
		fmt.Printf("Issues when saving template for dashboard %s/%s\n", bag.Namespace, bag.TierName)
	}
	updated, errGen := dw.generateDeploymentDashboard(dashboard, bag)
	fmt.Printf("generateDeploymentDashboard. %v\n", errGen)
	if errGen != nil {
		return fmt.Errorf("Issues when generating template for deployment dashboard %s, %s. %v\n", bag.Namespace, bag.TierName, err)
	}
	dw.saveTemplate(updated, fileName)
	fmt.Printf("Saving dashboard %.0f, %s\n", updated.ID, updated.Name)
	_, errSaveDash := dw.saveDashboard(updated)
	fmt.Printf("saveDashboard. %v\n", errSaveDash)

	return errSaveDash
}

func (dw *DashboardWorker) generateDeploymentDashboard(dashboard *m.Dashboard, bag *m.DashboardBag) (*m.Dashboard, error) {
	//	dashboard.WidgetTemplates = []m.WidgetTemplate{}
	fmt.Printf("generateDeploymentDashboard %s/%s\n", bag.Namespace, bag.TierName)
	d, err := dw.addBackground(dashboard, bag)
	if err != nil {
		fmt.Printf("Error when adding background to dash %s. %v\n", dashboard.Name, err)
		return nil, err
	}
	d, err = dw.addDeploymentSummaryWidget(dashboard, bag)
	if err != nil {
		fmt.Printf("Error when adding tier summary to dash %s. %v\n", dashboard.Name, err)
		return nil, err
	}
	fmt.Printf("Added %d widgets to dash %.0f, %s\n", len(d.Widgets), d.ID, d.Name)
	return d, err
}

func (dw *DashboardWorker) addBackground(dashboard *m.Dashboard, bag *m.DashboardBag) (*m.Dashboard, error) {
	widgetList, err, exists := dw.loadWidgetTemplate(BACKGROUND)
	if err != nil && exists {
		return nil, fmt.Errorf("Background template exists, but cannot be loaded. %v\n", err)
	}
	if !exists {
		return nil, fmt.Errorf("Background template does not exist, skipping dashboard for deployment %s/%s. %v\n", bag.Namespace, bag.TierName, err)
	}
	backgroundWidet := (*widgetList)[0]
	backgroundWidet["height"] = dashboard.Height - WIDGET_GAP
	backgroundWidet["dashboardId"] = dashboard.ID

	dashboard.Widgets = append(dashboard.Widgets, backgroundWidet)
	fmt.Printf("Added background %.0f x %.0f to dash %s\n", backgroundWidet["width"], backgroundWidet["weight"], dashboard.Name)
	return dashboard, nil
}

func (dw *DashboardWorker) addDeploymentSummaryWidget(dashboard *m.Dashboard, bag *m.DashboardBag) (*m.Dashboard, error) {
	widgetList, err, exists := dw.loadWidgetTemplate(TIER_SUMMARY)
	fmt.Printf("Loaded tier summary  %v. %t. Num widgets: %d\n", err, exists, len(*widgetList))
	if err != nil && exists {
		return nil, fmt.Errorf("Tier summary template exists, but cannot be loaded. %v\n", err)
	}
	if !exists {
		return nil, fmt.Errorf("Tier summary template does not exist, skipping dashboard for deployment %s/%s. %v\n", bag.Namespace, bag.TierName, err)
	}
	fmt.Printf("Adding tier summary to dash %s\n", dashboard.Name)
	var maxY float64 = 0
	for _, widget := range *widgetList {
		widget["id"] = 0
		appName, tierName := dw.GetEntityInfo(&widget, bag)
		fmt.Printf("Widget info: %s %s\n", appName, tierName)
		y := widget["y"].(float64)
		height := widget["height"].(float64)

		if y+height > maxY {
			maxY = y + height
		}

		widgetTitle, ok := widget["title"]
		if ok && widgetTitle != nil && strings.Contains(widgetTitle.(string), "%APP_TIER_NAME%") {
			widget["title"] = strings.Replace(widgetTitle.(string), "%APP_TIER_NAME%", bag.TierName, 1)
		}
		appID, okApp := widget["applicationId"]
		if okApp && appID != nil && strings.Contains(appID.(string), "%APP_ID%") {
			widget["applicationId"] = bag.AppID
		}

		entityIDs, hasEntityIDs := widget["entityIds"]
		if hasEntityIDs && entityIDs != nil && strings.Contains(entityIDs.(string), "%APP_TIER_ID%") {
			widget["entityIds"] = []int{bag.TierID}
		}

		dsTemplates, ok := widget["widgetsMetricMatchCriterias"]
		if ok && dsTemplates != nil {
			for _, t := range dsTemplates.([]interface{}) {
				if t != nil {
					mm, ok := t.(map[string]interface{})["metricMatchCriteria"]
					if ok && mm != nil {
						mmBlock := mm.(map[string]interface{})
						mmBlock["applicationId"] = bag.ClusterAppID
						exp, ok := mmBlock["metricExpression"]
						if ok && exp != nil {
							e := dw.updateMetricDefinition(exp.(map[string]interface{}), bag, &widget)
							if e != nil {
								return nil, e
							}
						}
					}
				}
			}
		}
	}

	dashboard.Height = maxY + WIDGET_GAP
	dashboard.Widgets = *widgetList
	fmt.Printf("Adjusted dashboard dimensions %d x %d\n", dashboard.Width, dashboard.Height)
	//adjust the background height
	dashboard.Widgets[0]["height"] = dashboard.Height
	fmt.Printf("Adjusted background dimensions %.0f x %.0f\n", dashboard.Widgets[0]["width"], dashboard.Widgets[0]["height"])
	fmt.Printf("Added tier summary to to dash %s\n", dashboard.Name)
	return dashboard, nil
}

func (dw *DashboardWorker) updateMetricDefinition(expTemplate map[string]interface{}, bag *m.DashboardBag, parentWidget *map[string]interface{}) error {
	ok, definition := dw.nodeHasDefinition(expTemplate)
	if ok {
		return dw.updateMetricName(definition, bag, parentWidget)
	} else {
		return dw.updateMetricsExpression(expTemplate, bag, parentWidget)
	}
}

func (dw *DashboardWorker) nodeHasDefinition(node map[string]interface{}) (bool, map[string]interface{}) {

	definition, ok := node["metricDefinition"]
	if ok && definition != nil {
		definitionObj := definition.(map[string]interface{})
		_, ok = definitionObj["logicalMetricName"]
		return ok, definitionObj
	}
	return ok && definition != nil, nil
}

func (dw *DashboardWorker) updateMetricName(expTemplate map[string]interface{}, dashBag *m.DashboardBag, parentWidget *map[string]interface{}) error {
	if expTemplate["logicalMetricName"] == nil {
		return nil
	}
	rawPath := expTemplate["logicalMetricName"].(string)
	metricPath := ""
	if dashBag.Type == m.Cluster {
		metricPath = fmt.Sprintf(rawPath, dw.Bag.TierName)
	}

	if dashBag.Type == m.Tier {
		metricPath = fmt.Sprintf(rawPath, dw.Bag.TierName, dashBag.Namespace, dashBag.TierName)
	}

	metricID, err := dw.AppdController.GetMetricID(dw.Bag.AppID, metricPath)
	if err != nil {
		return fmt.Errorf("Cannot get metric ID for %s. %v\n", metricPath, err)
	} else if metricID == 0 {
		return fmt.Errorf("Metrics are not fully registered with the controller %s. Delaying dashboard generation...\n", metricPath)
	} else {
		expTemplate["metricId"] = metricID
	}
	expTemplate["logicalMetricName"] = metricPath
	scopeBlock := expTemplate["scope"].(map[string]interface{})
	scopeBlock["entityId"] = dashBag.ClusterTierID

	//update drilldown if cluster level dash
	if dashBag.Type == m.Cluster {
		searchPath := dw.AdqlWorker.GetSearch(rawPath)
		if searchPath != "" && ((*parentWidget)["drillDownUrl"] == "" || (*parentWidget)["drillDownUrl"] == nil) {
			(*parentWidget)["drillDownUrl"] = searchPath
			(*parentWidget)["useMetricBrowserAsDrillDown"] = false
		}
	}
	return nil
}

func (dw *DashboardWorker) updateMetricsExpression(expTemplate map[string]interface{}, dashBag *m.DashboardBag, parentWidget *map[string]interface{}) error {
	for i := 0; i < 2; i++ {
		expNodeName := fmt.Sprintf("expression%d", i+1)
		if expNode, ok := expTemplate[expNodeName]; ok {
			hasDefinition, definition := dw.nodeHasDefinition(expNode.(map[string]interface{}))
			if hasDefinition {
				err := dw.updateMetricName(definition, dashBag, parentWidget)
				if err != nil {
					return err
				}
			} else {
				err := dw.updateMetricsExpression(expNode.(map[string]interface{}), dashBag, parentWidget)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

//templates

func (dw *DashboardWorker) updateDashNode(expTemplate map[string]interface{}, bag *m.DashboardBag, parentWidget *map[string]interface{}) {
	if dw.nodeHasPath(expTemplate) {
		dw.updateNodePath(expTemplate, bag, parentWidget)
	} else {
		dw.updateNodeExpression(expTemplate, bag, parentWidget)
	}
}

func (dw *DashboardWorker) nodeHasPath(node map[string]interface{}) bool {

	_, ok := node["metricPath"]

	return ok
}

func (dw *DashboardWorker) updateNodePath(expTemplate map[string]interface{}, dashBag *m.DashboardBag, parentWidget *map[string]interface{}) {
	expTemplate["metricPath"] = fmt.Sprintf(expTemplate["metricPath"].(string), dw.Bag.TierName, dashBag.Namespace, dashBag.TierName)
	scopeBlock := expTemplate["scopeEntity"].(map[string]interface{})
	appName, tierName := dw.GetEntityInfo(parentWidget, dashBag)
	scopeBlock["applicationName"] = appName
	scopeBlock["entityName"] = tierName

	//drill-down
}

func (dw *DashboardWorker) updateNodeExpression(expTemplate map[string]interface{}, dashBag *m.DashboardBag, parentWidget *map[string]interface{}) {
	for i := 0; i < 2; i++ {
		expNodeName := fmt.Sprintf("expression%d", i+1)
		if expNode, ok := expTemplate[expNodeName]; ok {
			if dw.nodeHasPath(expNode.(map[string]interface{})) {
				dw.updateNodePath(expNode.(map[string]interface{}), dashBag, parentWidget)
			} else {
				dw.updateNodeExpression(expNode.(map[string]interface{}), dashBag, parentWidget)
			}
		}
	}
}

func (dw *DashboardWorker) GetEntityInfo(widget *map[string]interface{}, dashBag *m.DashboardBag) (string, string) {
	label, ok := (*widget)["label"]
	isAPMWidget := ok && (label == "APM")
	appName := dw.Bag.AppName
	tierName := dw.Bag.TierName
	if isAPMWidget {
		appName = dashBag.AppName
		tierName = dashBag.TierName
	}
	return appName, tierName
}

func (dw *DashboardWorker) DeleteDashboard(id int) error {
	path := "restui/dashboards/deleteDashboards"
	arr := []int{id}
	data, err := json.Marshal(arr)
	if err != nil {
		return fmt.Errorf("Unable to serialize data to delete dashboard %d. %v", id, err)
	}
	rc := app.NewRestClient(dw.Bag, dw.Logger)
	_, errDel := rc.CallAppDController(path, "POST", data)
	if errDel != nil {
		return fmt.Errorf("Unable to delete dashboard %d. %v\n", id, errDel)
	}
	return nil
}
