# TODO : Convert taxonomies repository into package and use that instead!

import re
import networkx as nx
from typing import List


def load_taxonomy(
    file_path: str,
    return_all: List = False,
    verbose: bool = False,
    print_prefix: str = "",
) -> any:
    """
    Loads a taxonomy in the graphviz format:

        digraph taxonomy_001 {
            Reasoning -> Deduction;
            ...
            }
        digraph taxonomy_001_labeling {
            Reasoning -> LegalSupport;
            ...
            }

    Returns a networkx representation of it.
    """

    # Load both graphs, taxonomy and labels
    graphs_dict = dict()
    with open(file_path) as f:
        for line in f:
            if len(line) == 0 or re.search("\s+//", line):
                continue

            if "{" in line:
                graph_name = line.split("digraph")[-1].split("{")[0].strip()
                if verbose:
                    print(print_prefix + "Found graph : %s" % graph_name)
                graphs_dict[graph_name] = nx.DiGraph(name=graph_name)
            elif "}" in line:
                continue
            elif " -> " not in line:
                continue
            else:
                # Get nodes
                from_n = (
                    line.split(" -> ")[0].strip().replace(";", "").replace(":", "---")
                )
                to_n = (
                    line.split(" -> ")[-1].strip().replace(";", "").replace(":", "---")
                )
                # Add to graph (wont be duplicated)
                graphs_dict[graph_name].add_node(from_n)
                graphs_dict[graph_name].add_node(to_n)
                # Add edge
                graphs_dict[graph_name].add_edge(from_n, to_n)

    # Check taxonomy file in correct order and naming convention
    assert len(graphs_dict.keys()) == 2
    taxonomy_name = list(graphs_dict.keys())[0]
    assert taxonomy_name + "_labeling" == list(graphs_dict.keys())[1]

    # Add datasets to nodes in the taxonomy graph using the labels graph
    dataset_correspondency = dict()
    for edge in graphs_dict[taxonomy_name + "_labeling"].edges:
        if edge[0] not in dataset_correspondency.keys():
            dataset_correspondency[edge[0]] = list()
        dataset_correspondency[edge[0]].append(edge[1])
    taxonomy_graph = graphs_dict[taxonomy_name]
    labels_graph = graphs_dict[taxonomy_name + "_labeling"]
    nx.set_node_attributes(taxonomy_graph, dataset_correspondency, name="datasets")

    # Get the measurable edges, those with defined datasets in both nodes
    undefined_edges = list()
    measurable_edges = list()
    for edge in taxonomy_graph.edges:
        if (
            taxonomy_graph.nodes[edge[0]].get("datasets", None) is None
        ) or taxonomy_graph.nodes[edge[1]].get("datasets", None) is None:
            undefined_edges.append(edge)
        else:
            measurable_edges.append(edge)
    if verbose:
        print(
            print_prefix
            + "%d undefined edges of %d edges (%d are potentially measurable)"
            % (len(undefined_edges), len(taxonomy_graph.edges), len(measurable_edges))
        )

    # Check if the graph contains the same dataset in two nodes that are on the
    # same dependency path

    # Get nodes without outgoing connections
    base_nodes = [
        node for node, out_degree in taxonomy_graph.out_degree() if out_degree == 0
    ]

    def recursive_explore(node_path, dataset_list):
        """
        Given a node path and a list of datasets already assigned, checks if the
        incoming edges contain any of these datasets, if thats the case, it
        throws an error.
        For each incoming edge the function calls itself with the updated dataset
        and node path list. This is repeated until the root of the graph is found,
        which has no incoming edges.
        This works because taxonomies are rather small because they should be
        easily understood by humans.
        """
        # Get the node to analyze, the last from the given path
        node = node_path[-1]
        for edge in taxonomy_graph.in_edges(node):
            if node != edge[1]:
                # We don't care on outgoing edges from the analyzed node.
                continue
            else:
                # Get list of datasets used here
                dataset_list_aux = taxonomy_graph.nodes[edge[0]].get("datasets", [])
                for dataset in dataset_list_aux:
                    if dataset in dataset_list:
                        print(print_prefix + "Error in path : ")
                        for node in node_path:
                            print(print_prefix + "\t%s" % node_path)
                        raise ValueError(
                            "Detected downstream dataset sharing in node %s with %s on dataset %s"
                            % (node, edge[0], dataset)
                        )
                # Go deeper
                recursive_explore(
                    node_path + [edge[0]], dataset_list + dataset_list_aux
                )
        return

    # For each node, go up and make sure no dataset is shared among its paths up
    for node in base_nodes:
        # Explore path
        recursive_explore([node], taxonomy_graph.nodes[node].get("datasets", []))

    # All ok, return graph
    if return_all:
        return taxonomy_graph, labels_graph, undefined_edges, measurable_edges
    else:
        return taxonomy_graph
