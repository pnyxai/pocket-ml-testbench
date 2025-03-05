from lm_eval.filters.extraction import Filter
from lm_eval.api.registry import register_filter

import re


@register_filter("GetResponse")
class GetResponse(Filter):
    """ """

    def apply(self, resps, docs):
        filtered_resps = []

        for r, doc in zip(resps, docs):
            filtered = []
            for resp in r:
                if "</think>" in resp:
                    # Remove CoT content
                    resp = resp.split("</think>")[-1]
                else:
                    # Remove everything after double line jump
                    resp = resp.split("\n\n")[0]
                # Remove leading white spaces
                resp = resp.lstrip()
                # function to ignore right white spaces or line breaks
                resp = re.sub(r"\s+$", "", resp)
                # If there are things  between brackets, match those
                search = re.search("\(([^)]+)\)", resp)
                if search is not None:
                    resp = search.groups(0)[0]

                filtered.append(resp)
            filtered_resps.append(filtered)

        return filtered_resps
