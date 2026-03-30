package vk

type vkGetWallResponse struct {
	Response struct {
		Items []vkWallPost `json:"items"`
	} `json:"response"`
}

type vkResolveNameResponse struct {
	Response struct {
		ID int `json:"object_id"`
	} `json:"response"`
}

type vkGetCommentsResponse struct {
	Response struct {
		Items []vkComment `json:"items"`
	} `json:"response"`
}
