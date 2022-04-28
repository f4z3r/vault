import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
/**
 * @module Attribution
 * Attribution components display the top 10 total client counts for namespaces or auth methods (mounts) during a billing period.
 * A horizontal bar chart shows on the right, with the top namespace/auth method and respective client totals on the left.
 *
 * @example
 * ```js
 *  <Clients::Attribution
 *    @chartLegend={{this.chartLegend}}
 *    @totalUsageCounts={{this.totalUsageCounts}}
 *    @newUsageCounts={{this.newUsageCounts}}
 *    @totalClientAttribution={{this.totalClientAttribution}}
 *    @newClientAttribution={{this.newClientAttribution}}
 *    @selectedNamespace={{this.selectedNamespace}}
 *    @startTimeDisplay={{date-format this.responseTimestamp "MMMM yyyy"}}
 *    @isDateRange={{this.isDateRange}}
 *    @isCurrentMonth={{false}}
 *    @timestamp={{this.responseTimestamp}}
 *  />
 * ```
 * @param {array} chartLegend - (passed to child) array of objects with key names 'key' and 'label' so data can be stacked
 * @param {object} totalUsageCounts - object with total client counts for chart tooltip text
 * @param {object} newUsageCounts - object with new client counts for chart tooltip text
 * @param {array} totalClientAttribution - array of objects containing a label and breakdown of client counts for total clients
 * @param {array} newClientAttribution - array of objects containing a label and breakdown of client counts for new clients
 * @param {string} selectedNamespace - namespace selected from filter bar
 * @param {string} startTimeDisplay - string that displays as start date for CSV modal
 * @param {string} endTimeDisplay - string that displays as end date for CSV modal
 * @param {boolean} isDateRange - getter calculated in parent to relay if dataset is a date range or single month and display text accordingly
 * @param {boolean} isCurrentMonth - boolean to determine if rendered in current month tab or not
 * @param {string} timestamp -  ISO timestamp created in serializer to timestamp the response
 */

export default class Attribution extends Component {
  @tracked showCSVDownloadModal = false;
  @service downloadCsv;
  @service store;

  get hasCsvData() {
    return this.args.totalClientAttribution ? this.args.totalClientAttribution.length > 0 : false;
  }
  get isDateRange() {
    return this.args.isDateRange;
  }

  get isSingleNamespace() {
    if (!this.args.totalClientAttribution) {
      return 'no data';
    }
    // if a namespace is selected, then we're viewing top 10 auth methods (mounts)
    return !!this.args.selectedNamespace;
  }

  // truncate data before sending to chart component
  get barChartTotalClients() {
    return this.args.totalClientAttribution?.slice(0, 10);
  }

  get barChartNewClients() {
    return this.args.newClientAttribution?.slice(0, 10);
  }

  get topClientCounts() {
    // get top namespace or auth method
    return this.args.totalClientAttribution ? this.args.totalClientAttribution[0] : null;
  }

  get attributionBreakdown() {
    // display text for hbs
    return this.isSingleNamespace ? 'auth method' : 'namespace';
  }

  get chartText() {
    let dateText = this.isDateRange ? 'date range' : 'month';
    switch (this.isSingleNamespace) {
      case true:
        return {
          description:
            'This data shows the top ten authentication methods by client count within this namespace, and can be used to understand where clients are originating. Authentication methods are organized by path.',
          newCopy: `The new clients used by the auth method for this ${dateText}. This aids in understanding which auth methods create and use new clients${
            dateText === 'date range' ? ' over time.' : '.'
          }`,
          totalCopy: `The total clients used by the auth method for this ${dateText}. This number is useful for identifying overall usage volume. `,
        };
      case false:
        return {
          description:
            'This data shows the top ten namespaces by client count and can be used to understand where clients are originating. Namespaces are identified by path. To see all namespaces, export this data.',
          newCopy: `The new clients in the namespace for this ${dateText}.
          This aids in understanding which namespaces create and use new clients${
            dateText === 'date range' ? ' over time.' : '.'
          }`,
          totalCopy: `The total clients in the namespace for this ${dateText}. This number is useful for identifying overall usage volume.`,
        };
      case 'no data':
        return {
          description: 'There is a problem gathering data',
        };
      default:
        return '';
    }
  }

  destructureCountsToArray(object) {
    let { clients, entity_clients, non_entity_clients } = object;
    return [clients, entity_clients, non_entity_clients];
  }

  constructCsvRow(firstColumn, secondColumn = null, otherColumns) {
    // ['ns label', 'mount label', 'total clients', 'entity', 'non-entity']
    // if constructing namespace row, mount column will be blank
    // if firstColumn is a string, then we're at mount level attribution
    return [
      `${typeof firstColumn === 'string' ? firstColumn : firstColumn.label}`,
      `${secondColumn ? secondColumn.label : ''}`,
      ...otherColumns,
    ];
  }
  generateCsvData() {
    let graphData = this.args.totalClientAttribution;
    let csvData = [],
      csvHeader = [
        'Namespace path',
        'Authentication method',
        'Total clients',
        'Entity clients',
        'Non-entity clients',
      ];

    // each array is a row in the csv file
    graphData.forEach((data) => {
      let namespace = this.isSingleNamespace ? this.args.selectedNamespace : data;
      let mount = this.isSingleNamespace ? data : null;
      let otherColumns = this.destructureCountsToArray(data);
      csvData.push(this.constructCsvRow(namespace, mount, otherColumns));
      if (!this.isSingleNamespace && namespace.mounts) {
        namespace.mounts.forEach((mount) => {
          csvData.push(this.constructCsvRow(namespace, mount, otherColumns));
        });
      }
    });
    csvData.unshift(csvHeader);
    // make each nested array a comma separated string, join each array "row" in csvData with line break (\n)
    return csvData.map((d) => d.join()).join('\n');
  }

  get getCsvFileName() {
    let endRange = this.isDateRange ? `-${this.args.endTimeDisplay}` : '';
    let csvDateRange = this.args.startTimeDisplay + endRange;
    return this.isSingleNamespace
      ? `clients_by_auth_method_${csvDateRange}`
      : `clients_by_namespace_${csvDateRange}`;
  }

  // ACTIONS
  @action
  exportChartData(filename) {
    let contents = this.generateCsvData();
    this.downloadCsv.download(filename, contents);
    this.showCSVDownloadModal = false;
  }
}
