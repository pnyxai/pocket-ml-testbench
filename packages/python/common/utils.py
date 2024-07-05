def deep_update(source, overrides):
    for key, value in overrides.items():
        if isinstance(value, dict):
            # Get node or create one
            node = source.setdefault(key, {})
            deep_update(node, value)
        else:
            source[key] = value

    return source


def get_from_dict(data_dict, path, default=None):
    """
    :param data_dict: The given data dictionary.
    :param path: The path to the requested data.
    :param default: The default value to return if the path is not found in the data dictionary.
    :return: The data at the given path in the data dictionary, or the default value if the path is not found.
    """
    keys = path.split(".")
    result = data_dict
    for key in keys:
        try:
            result = result[key]
        except KeyError:
            return default
    return result
