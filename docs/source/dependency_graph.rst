Dependency Graph
================

To visualize Biscuit's module dependencies, run ``go mod graph`` and convert
the output to Graphviz `dot` format. The ``misc/depgraph`` helper automates this:

.. code-block:: console

   $ go run ./misc/depgraph > deps.dot
   $ dot -Tpng deps.dot -o deps.png

Open ``deps.png`` to view the dependency graph.
